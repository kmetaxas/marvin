package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/journal"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// journalQueryTask implements the linux.journal.query capability.
type journalQueryTask struct{ provider *Provider }

const journalQuerySchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.journal.query Parameters",
  "description": "Parameters for linux.journal.query.",
  "properties": {
    "service": {
      "type": "string",
      "description": "Systemd service unit name filter"
    },
    "priority": {
      "type": "string",
      "description": "Priority ceiling as a name (emerg, alert, crit, err, warning, notice, info, debug) or a number 0-7; entries above the ceiling are excluded"
    },
    "since": {
      "type": "string",
      "description": "Start timestamp (RFC3339, YYYY-MM-DDTHH:MM:SS, or YYYY-MM-DD)"
    },
    "until": {
      "type": "string",
      "description": "End timestamp (RFC3339, YYYY-MM-DDTHH:MM:SS, or YYYY-MM-DD)"
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of journal entries to return (default 50, capped at 500)"
    }
  }
}`

func (t *journalQueryTask) Name() string       { return "linux.journal.query" }
func (t *journalQueryTask) JSONSchema() string { return journalQuerySchema }

func (t *journalQueryTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("journal.query starting", "capability", t.Name())

	service, err := common.OptionalString(params, "service", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	priority, err := common.OptionalString(params, "priority", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	since, err := common.OptionalString(params, "since", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	until, err := common.OptionalString(params, "until", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := journalEntryLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	ceiling, err := parsePriorityCeiling(priority)
	if err != nil {
		return common.TaskFailure(err)
	}
	sinceTime, err := parseJournalTime(since)
	if err != nil {
		return common.TaskFailure(err)
	}
	untilTime, err := parseJournalTime(until)
	if err != nil {
		return common.TaskFailure(err)
	}

	cfg := t.provider.CurrentConfig()
	files, err := listJournalFiles(t.provider.CurrentReader(), cfg.JournalDirectory, service)
	if err != nil {
		slog.Info("journal.query failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	reader, err := journal.Open(journalDirs(cfg.JournalDirectory))
	if err != nil {
		slog.Info("journal.query failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}
	defer reader.Close()

	entries, err := reader.Query(ctx, journal.Query{
		MatchUnit:   service,
		PriorityMax: ceiling,
		Since:       sinceTime,
		Until:       untilTime,
		Limit:       limit,
	})
	if err != nil {
		slog.Info("journal.query failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"files":    files,
		"service":  service,
		"priority": priority,
		"since":    since,
		"until":    until,
		"limit":    limit,
		"entries":  journalEntries(entries),
		"count":    len(entries),
	}
	slog.Info("journal.query succeeded", "capability", t.Name(), "count", len(entries))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*journalQueryTask)(nil)

// journalEntryLimit extracts the optional "limit" parameter: default 50,
// capped at 500, must be positive.
func journalEntryLimit(params map[string]any) (int, error) {
	limit, err := common.OptionalInt(params, "limit", 50)
	if err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, fmt.Errorf("parameter limit must be greater than 0")
	}
	if limit > 500 {
		limit = 500
	}
	return limit, nil
}

// journalDirs returns the journal directories to scan: the configured
// directory (defaulting to /var/log/journal) plus the runtime directory.
func journalDirs(cfgDir string) []string {
	if cfgDir == "" {
		cfgDir = "/var/log/journal"
	}
	return []string{cfgDir, "/run/log/journal"}
}

// journalPriorityLevels maps syslog priority names to numeric levels.
var journalPriorityLevels = map[string]int{
	"emerg":   0,
	"alert":   1,
	"crit":    2,
	"err":     3,
	"warning": 4,
	"notice":  5,
	"info":    6,
	"debug":   7,
}

// parsePriorityCeiling converts a priority parameter to a numeric ceiling.
// An empty value yields -1 (no filter).
func parsePriorityCeiling(value string) (int, error) {
	if value == "" {
		return -1, nil
	}
	if level, ok := journalPriorityLevels[strings.ToLower(value)]; ok {
		return level, nil
	}
	if level, err := strconv.Atoi(value); err == nil && level >= 0 && level <= 7 {
		return level, nil
	}
	return 0, fmt.Errorf("invalid priority %q: expected emerg, alert, crit, err, warning, notice, info, debug, or 0-7", value)
}

// journalTimeLayouts are the accepted timestamp formats (bare formats are UTC).
var journalTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// parseJournalTime parses a timestamp parameter. An empty value yields nil.
func parseJournalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	for _, layout := range journalTimeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			ts := parsed.UTC()
			return &ts, nil
		}
	}
	return nil, fmt.Errorf("invalid timestamp %q: expected RFC3339, YYYY-MM-DDTHH:MM:SS, or YYYY-MM-DD", value)
}

// journalEntries converts adapter entries to the structured result form.
func journalEntries(entries []journal.Entry) []JournalEntry {
	out := make([]JournalEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, journalEntryFrom(e))
	}
	return out
}

// journalEntryFrom maps one adapter entry onto JournalEntry.
func journalEntryFrom(e journal.Entry) JournalEntry {
	out := JournalEntry{
		Timestamp:  e.Timestamp.UTC().Format(time.RFC3339),
		Message:    e.Fields["MESSAGE"],
		Priority:   e.Fields["PRIORITY"],
		Unit:       e.Fields["_SYSTEMD_UNIT"],
		Identifier: e.Fields["SYSLOG_IDENTIFIER"],
		BootID:     e.Fields["_BOOT_ID"],
		Fields:     e.Fields,
	}
	if pid, err := strconv.Atoi(e.Fields["_PID"]); err == nil {
		out.PID = pid
	}
	if uid, err := strconv.Atoi(e.Fields["_UID"]); err == nil {
		out.UID = uid
	}
	return out
}

// journalFile describes a journal file discovered on disk.
type journalFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time,omitempty"`
}

// listJournalFiles scans the journal directory for .journal and .journal~
// files, optionally filtering by a service name. It does not parse the binary
// journal format.
func listJournalFiles(r *procfs.Reader, journalDir, service string) ([]journalFile, error) {
	if journalDir == "" {
		journalDir = "/var/log/journal"
	}

	var files []journalFile

	collect := func(base string) {
		machineIDs, err := r.ReadDirNames(base)
		if err != nil {
			return
		}
		for _, machineID := range machineIDs {
			machineDir := filepath.Join(base, machineID)
			if !r.IsDir(machineDir) {
				continue
			}
			names, err := r.ReadDirNames(machineDir)
			if err != nil {
				continue
			}
			for _, name := range names {
				if !strings.HasSuffix(name, ".journal") && !strings.HasSuffix(name, ".journal~") {
					continue
				}
				if service != "" && !strings.Contains(name, service) {
					continue
				}
				fullPath := r.Path(machineDir, name)
				info, err := os.Stat(fullPath)
				if err != nil {
					continue
				}
				files = append(files, journalFile{
					Name:    name,
					Path:    fullPath,
					Size:    info.Size(),
					ModTime: info.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00"),
				})
			}
		}
	}

	collect(journalDir)
	collect("/run/log/journal")

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	return files, nil
}
