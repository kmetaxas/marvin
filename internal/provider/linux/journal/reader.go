// Package journal adapts the pure-Go systemd journal SDK to Marvin's task
// layer. It provides discovery of journal files on disk, structured entry
// queries and boot enumeration without CGO and without invoking system tools.
package journal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sdjournal "github.com/netdata/systemd-journal-sdk/go/journal"
)

const (
	// maxEntries is the hard cap applied to Query results regardless of the
	// caller-supplied limit.
	maxEntries = 1000

	// ctxCheckInterval is how often (in iteration steps) Query checks the
	// context for cancellation. The underlying SDK has no context support.
	ctxCheckInterval = 256
)

// Entry is one journal entry with flattened fields.
type Entry struct {
	Timestamp time.Time
	Realtime  uint64            // microseconds
	Fields    map[string]string // first value per field
	Cursor    string
}

// Boot describes one boot known to the journal.
type Boot struct {
	ID         string
	FirstEntry time.Time // zero if unknown
	LastEntry  time.Time // zero if unknown
}

// Query filters journal entries. Matches are applied exactly on fields.
type Query struct {
	MatchUnit   string // exact _SYSTEMD_UNIT value, optional
	PriorityMax int    // ceiling 0..7, or -1 for no filter
	Since       *time.Time
	Until       *time.Time
	Limit       int // >0, caller-enforced cap; adapter hard-caps at 1000 entries
}

// Reader reads entries from a set of systemd journal files.
// The zero value is an empty reader and is safe to Close.
type Reader struct {
	dr *sdjournal.DirectoryReader
}

// Open discovers journal files in the given directories and opens them for
// reading. Nonexistent directories are skipped silently.
func Open(dirs []string) (*Reader, error) {
	files := discoverJournalFiles(dirs)
	dr, err := openFiles(files)
	if err != nil {
		return nil, err
	}
	return &Reader{dr: dr}, nil
}

// discoverJournalFiles finds journal files under the given directories. For
// each directory it collects *.journal and *.journal~ entries directly in the
// directory plus one level of machine-id style subdirectories (32-36 hex
// characters, no dot; namespaced <machine-id>.<ns> directories are skipped).
// Results are deduplicated by absolute cleaned path.
func discoverJournalFiles(dirs []string) []string {
	seen := make(map[string]bool)
	var files []string

	add := func(path string) {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		abs = filepath.Clean(abs)
		if seen[abs] {
			return
		}
		seen[abs] = true
		files = append(files, abs)
	}

	collectDir := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			slog.Debug("journal: skipping directory", "dir", dir, "error", err)
			return
		}
		for _, e := range entries {
			name := e.Name()
			full := filepath.Join(dir, name)
			switch {
			case e.Type().IsRegular() && isJournalFileName(name):
				add(full)
			case e.IsDir() && isMachineIDName(name):
				sub, err := os.ReadDir(full)
				if err != nil {
					slog.Debug("journal: skipping subdirectory", "dir", full, "error", err)
					continue
				}
				for _, se := range sub {
					if se.Type().IsRegular() && isJournalFileName(se.Name()) {
						add(filepath.Join(full, se.Name()))
					}
				}
			}
		}
	}

	for _, dir := range dirs {
		collectDir(dir)
	}
	return files
}

// isJournalFileName reports whether name looks like a systemd journal file.
func isJournalFileName(name string) bool {
	return strings.HasSuffix(name, ".journal") || strings.HasSuffix(name, ".journal~")
}

// isMachineIDName reports whether name looks like a machine-id directory:
// 32-36 hex characters (dashed UUID form allowed) and no dot. Namespaced
// <machine-id>.<ns> directories contain a dot and are rejected.
func isMachineIDName(name string) bool {
	if len(name) < 32 || len(name) > 36 {
		return false
	}
	for _, ch := range name {
		if ch == '.' {
			return false
		}
		if ch != '-' && !isHexDigit(ch) {
			return false
		}
	}
	return true
}

func isHexDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// openFiles opens the given journal files. If opening the full set fails
// (for example because one file has a torn, unpublished tail), each file is
// probed individually, failures are skipped with a warning, and the remaining
// files are opened. If nothing remains, an empty reader is returned.
func openFiles(paths []string) (*sdjournal.DirectoryReader, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	dr, err := sdjournal.OpenFiles(paths)
	if err == nil {
		return dr, nil
	}

	var remaining []string
	for _, path := range paths {
		probe, perr := sdjournal.OpenFile(path)
		if perr != nil {
			slog.Warn("journal: skipping unreadable journal file", "path", path, "error", perr)
			continue
		}
		_ = probe.Close()
		remaining = append(remaining, path)
	}
	if len(remaining) == 0 {
		return nil, nil
	}
	return sdjournal.OpenFiles(remaining)
}

// Query returns matching entries, newest first. Entries are filtered by the
// exact _SYSTEMD_UNIT value (when set) and the priority ceiling (when >= 0,
// entries without a PRIORITY field are excluded). The adapter hard-caps the
// number of returned entries at 1000.
func (r *Reader) Query(ctx context.Context, q Query) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries := []Entry{}
	if r == nil || r.dr == nil {
		return entries, nil
	}

	limit := q.Limit
	if limit <= 0 || limit > maxEntries {
		limit = maxEntries
	}

	dr := r.dr
	dr.FlushMatches()
	if q.MatchUnit != "" {
		match, err := sdjournal.ParseMatchString("_SYSTEMD_UNIT=" + q.MatchUnit)
		if err != nil {
			return nil, fmt.Errorf("invalid unit match %q: %w", q.MatchUnit, err)
		}
		dr.AddMatch(match)
	}

	var hasUntil bool
	var untilUsec uint64
	if q.Until != nil {
		hasUntil = true
		untilUsec = uint64(q.Until.UnixMicro())
	}

	if q.Since != nil {
		// Forward from Since, bounded by Until, then reversed to newest-first.
		if err := dr.SeekRealtimeUsec(uint64(q.Since.UnixMicro())); err != nil {
			return nil, err
		}
		var collected []Entry
		for steps := 0; len(collected) < limit; steps++ {
			if steps%ctxCheckInterval == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			ok, err := dr.Step()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}
			e, err := dr.GetEntry()
			if err != nil {
				return nil, err
			}
			if hasUntil && e.Realtime > untilUsec {
				break
			}
			if !withinPriority(e, q.PriorityMax) {
				continue
			}
			collected = append(collected, convertEntry(e))
		}
		for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
			collected[i], collected[j] = collected[j], collected[i]
		}
		return collected, nil
	}

	// Tail walk backwards from the newest entry.
	if err := dr.SeekTail(); err != nil {
		return nil, err
	}
	for steps := 0; len(entries) < limit; steps++ {
		if steps%ctxCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		ok, err := dr.StepBack()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		e, err := dr.GetEntry()
		if err != nil {
			return nil, err
		}
		if hasUntil && e.Realtime > untilUsec {
			// Walking backwards: skip entries newer than the bound.
			continue
		}
		if !withinPriority(e, q.PriorityMax) {
			continue
		}
		entries = append(entries, convertEntry(e))
	}
	return entries, nil
}

// Boots returns the boots known to the journal, in SDK order (oldest first),
// deduplicated by boot ID keeping the first occurrence.
func (r *Reader) Boots(ctx context.Context) ([]Boot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	boots := []Boot{}
	if r == nil || r.dr == nil {
		return boots, nil
	}

	infos, err := r.dr.ListBoots()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(infos))
	for _, info := range infos {
		if seen[info.BootID] {
			continue
		}
		seen[info.BootID] = true

		boot := Boot{ID: info.BootID}
		if info.FirstEntry != 0 {
			boot.FirstEntry = time.UnixMicro(info.FirstEntry).UTC()
		}
		if info.LastEntry != 0 {
			boot.LastEntry = time.UnixMicro(info.LastEntry).UTC()
		}
		boots = append(boots, boot)
	}
	return boots, nil
}

// Close closes the underlying journal reader. It is safe on a zero-value
// Reader and on a nil receiver.
func (r *Reader) Close() error {
	if r == nil || r.dr == nil {
		return nil
	}
	dr := r.dr
	r.dr = nil
	return dr.Close()
}

// withinPriority reports whether the entry satisfies the priority ceiling.
// With a ceiling of >= 0 the entry must carry a parseable PRIORITY field no
// greater than the ceiling (journalctl -p semantics).
func withinPriority(e *sdjournal.Entry, ceiling int) bool {
	if ceiling < 0 {
		return true
	}
	raw, ok := e.Fields["PRIORITY"]
	if !ok {
		return false
	}
	priority, err := strconv.Atoi(string(raw))
	if err != nil {
		return false
	}
	return priority <= ceiling
}

// convertEntry flattens an SDK entry into the adapter's Entry form.
func convertEntry(e *sdjournal.Entry) Entry {
	fields := make(map[string]string, len(e.Fields))
	for name, value := range e.Fields {
		fields[name] = string(value)
	}
	return Entry{
		Timestamp: time.UnixMicro(int64(e.Realtime)).UTC(),
		Realtime:  e.Realtime,
		Fields:    fields,
		Cursor:    e.Cursor,
	}
}
