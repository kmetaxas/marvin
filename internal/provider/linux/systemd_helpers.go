package linux

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
)

// systemdUnitSearchDirs lists the standard systemd unit search paths, expressed
// as path elements relative to the procfs Reader root. Order matches systemd's
// own unit load precedence (drop-in/run first, vendor last).
var systemdUnitSearchDirs = [][]string{
	{"etc", "systemd", "system"},
	{"run", "systemd", "system"},
	{"usr", "lib", "systemd", "system"},
	{"lib", "systemd", "system"},
}

// systemdUnitSuffixes are the unit file suffixes we recognize when listing.
var systemdUnitSuffixes = []string{
	".service", ".socket", ".target", ".mount",
	".timer", ".path", ".slice", ".scope",
}

// systemdUnitType returns the unit type (suffix without the leading dot), e.g.
// "sshd.service" -> "service", "multi-user.target" -> "target".
func systemdUnitType(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 || idx == len(name)-1 {
		return ""
	}
	return name[idx+1:]
}

// hasSystemdUnitSuffix reports whether name ends in a known unit suffix.
func hasSystemdUnitSuffix(name string) bool {
	for _, suffix := range systemdUnitSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// parseSystemdUnit parses an INI-style systemd unit file into a map of section
// name -> key -> value. Comments (leading # or ;) and blank lines are skipped.
func parseSystemdUnit(content string) map[string]map[string]string {
	sections := make(map[string]map[string]string)
	current := ""

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSpace(line[1 : len(line)-1])
			if _, ok := sections[current]; !ok {
				sections[current] = make(map[string]string)
			}
			continue
		}
		if current == "" {
			continue
		}
		if idx := strings.Index(line, "="); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			sections[current][key] = val
		}
	}
	return sections
}

// unitPIDFile extracts the PIDFile directive from a unit file's [Service]
// section. Returns an empty string if absent or if it contains a specifier
// (e.g. %n) that cannot be resolved without systemd.
func unitPIDFile(sections map[string]map[string]string) string {
	service, ok := sections["Service"]
	if !ok {
		return ""
	}
	pidFile := strings.TrimSpace(service["PIDFile"])
	if pidFile == "" || strings.Contains(pidFile, "%") {
		return ""
	}
	return pidFile
}

// pidFileElements converts an absolute path like "/run/sshd.pid" into path
// elements relative to the Reader root.
func pidFileElements(pidFile string) []string {
	return strings.Split(strings.TrimPrefix(pidFile, "/"), "/")
}

// unitActive reports whether a unit appears active using filesystem-only
// signals: a symlink under /run/systemd/units/, a cgroup directory under
// /sys/fs/cgroup/system.slice/ (services), or the existence of its PID file.
func unitActive(r *procfs.Reader, name string, sections map[string]map[string]string) bool {
	if r.Exists("run", "systemd", "units", name) {
		return true
	}
	if strings.HasSuffix(name, ".service") && r.IsDir("sys", "fs", "cgroup", "system.slice", name) {
		return true
	}
	if pidFile := unitPIDFile(sections); pidFile != "" {
		return r.Exists(pidFileElements(pidFile)...)
	}
	return false
}

// unitMainPID returns the PID recorded in the unit's PIDFile, or 0 if the file
// is absent or unreadable.
func unitMainPID(r *procfs.Reader, sections map[string]map[string]string) int {
	pidFile := unitPIDFile(sections)
	if pidFile == "" {
		return 0
	}
	v, err := r.ReadInt(pidFileElements(pidFile)...)
	if err != nil {
		return 0
	}
	return v
}

// unitEnabled reports whether a unit is enabled by checking for a symlink
// pointing to it under any *.wants or *.requires directory in
// /etc/systemd/system/.
func unitEnabled(r *procfs.Reader, name string) bool {
	dirs, err := r.ReadDirNames("etc", "systemd", "system")
	if err != nil {
		return false
	}
	for _, d := range dirs {
		if !strings.HasSuffix(d, ".wants") && !strings.HasSuffix(d, ".requires") {
			continue
		}
		if r.Exists("etc", "systemd", "system", d, name) {
			return true
		}
	}
	return false
}

// findUnitFile locates a unit file across the standard systemd search paths and
// returns its absolute path and raw content. It is the shared entry point used
// by the unit show/definition/dependencies capabilities.
func findUnitFile(r *procfs.Reader, unit string) (string, []byte, error) {
	for _, dir := range systemdUnitSearchDirs {
		elems := append(append([]string{}, dir...), unit)
		if !r.Exists(elems...) {
			continue
		}
		path := r.Path(elems...)
		content, err := os.ReadFile(path)
		if err != nil {
			return "", nil, fmt.Errorf("read unit %q: %w", unit, err)
		}
		return path, content, nil
	}
	return "", nil, fmt.Errorf("unit %q not found in systemd unit directories", unit)
}

// parseUnitFile parses an INI-style systemd unit file into a map of section
// name -> key -> value. Comments (leading # or ;) and blank lines are skipped.
// Multi-line values (lines ending with a backslash) are joined with a single
// space, matching systemd's own continuation semantics.
func parseUnitFile(content []byte) map[string]map[string]string {
	sections := make(map[string]map[string]string)
	current := ""
	var pendingKey string

	flush := func() {
		if current != "" && pendingKey != "" {
			sections[current][pendingKey] = strings.TrimSpace(sections[current][pendingKey])
			pendingKey = ""
		}
	}

	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			current = strings.TrimSpace(line[1 : len(line)-1])
			if _, ok := sections[current]; !ok {
				sections[current] = make(map[string]string)
			}
			continue
		}
		if current == "" {
			continue
		}
		if idx := strings.Index(line, "="); idx >= 0 {
			flush()
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if _, ok := sections[current]; !ok {
				sections[current] = make(map[string]string)
			}
			sections[current][key] = val
			pendingKey = key
			continue
		}
		// Continuation line (no '='): append to the pending key's value.
		if pendingKey != "" {
			sections[current][pendingKey] += " " + line
		}
	}
	flush()

	// Strip trailing backslash continuation markers and collapse the joined
	// value into a single space-separated string.
	for section, kv := range sections {
		for key, val := range kv {
			val = strings.ReplaceAll(val, "\\", " ")
			sections[section][key] = strings.Join(strings.Fields(val), " ")
		}
	}
	return sections
}

// listUnitDropIns returns the drop-in files for a unit under
// /etc/systemd/system/<unit>.d/, sorted by name. Each entry carries the file
// name and its raw content.
func listUnitDropIns(r *procfs.Reader, unit string) []unitDropIn {
	dir := []string{"etc", "systemd", "system", unit + ".d"}
	names, err := r.ReadDirNames(dir...)
	if err != nil {
		return nil
	}
	sort.Strings(names)

	var out []unitDropIn
	for _, name := range names {
		if !strings.HasSuffix(name, ".conf") {
			continue
		}
		elems := append(append([]string{}, dir...), name)
		content, err := r.ReadFileString(elems...)
		if err != nil {
			continue
		}
		out = append(out, unitDropIn{Name: name, Content: content})
	}
	return out
}

// unitDropIn describes a single systemd drop-in file.
type unitDropIn struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
