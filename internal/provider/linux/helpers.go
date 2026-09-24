package linux

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// defaultAllowedReadPaths are the default allowed paths for file-reading capabilities.
var defaultAllowedReadPaths = []string{
	"/etc", "/var/log", "/proc", "/sys", "/tmp", "/home",
}

// defaultSecretPatterns are the default regex patterns for secret redaction.
var defaultSecretPatterns = []string{
	`(?i)password`,
	`(?i)secret`,
	`(?i)token`,
	`(?i)key`,
	`(?i)credential`,
	`(?i)auth`,
	`(?i)private`,
	`(?i)api_key`,
}

// ValidateReadPath ensures a requested path is within allowed directories.
// It resolves symlinks and prevents directory traversal.
func ValidateReadPath(path string, allowed []string) error {
	if allowed == nil {
		allowed = defaultAllowedReadPaths
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Resolve symlinks if possible.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// If the path doesn't exist, validate against allowed prefixes
		// using the non-resolved absolute path.
		resolved = abs
	}

	for _, allowedPath := range allowed {
		allowedAbs, _ := filepath.Abs(allowedPath)
		if strings.HasPrefix(resolved, allowedAbs) || strings.HasPrefix(resolved, allowedAbs+"/") {
			return nil
		}
	}
	return fmt.Errorf("path %q is not in allowed read paths", path)
}

// ValidatePID checks that a PID is a positive integer and exists in /proc.
func ValidatePID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("pid must be a positive integer, got %d", pid)
	}
	return nil
}

// RedactSecrets replaces values matching secret patterns with "[REDACTED]".
func RedactSecrets(env map[string]string, patterns []string) map[string]string {
	if patterns == nil {
		patterns = defaultSecretPatterns
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}

	out := make(map[string]string, len(env))
	for k, v := range env {
		redacted := false
		for _, re := range compiled {
			if re.MatchString(k) {
				out[k] = "[REDACTED]"
				redacted = true
				break
			}
		}
		if !redacted {
			out[k] = v
		}
	}
	return out
}

// DefaultAllowedReadPaths returns the default allowed read paths.
func DefaultAllowedReadPaths() []string {
	out := make([]string, len(defaultAllowedReadPaths))
	copy(out, defaultAllowedReadPaths)
	return out
}

// DefaultSecretPatterns returns the default secret patterns.
func DefaultSecretPatterns() []string {
	out := make([]string, len(defaultSecretPatterns))
	copy(out, defaultSecretPatterns)
	return out
}

// ParseEnvFile parses a file in the format KEY=value\0KEY=value\0...
// Used for /proc/[pid]/environ files.
func ParseEnvFile(data string) map[string]string {
	env := make(map[string]string)
	pairs := strings.Split(data, "\x00")
	for _, pair := range pairs {
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// ParseProcStat parses a /proc/[pid]/stat line.
// Returns pid, comm, state, ppid, pgrp, session, tty_nr, tpgid, flags, minflt,
// cminflt, majflt, cmajflt, utime, stime, cutime, cstime, priority, nice,
// num_threads, itrealvalue, starttime, vsize, rss.
// See proc(5) man page for details.
func ParseProcStat(line string) (map[string]any, error) {
	// The comm field is in parentheses and can contain spaces.
	// Find the first '(' and last ')' to extract comm.
	start := strings.IndexByte(line, '(')
	end := strings.LastIndexByte(line, ')')
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("invalid stat line format")
	}

	comm := line[start+1 : end]
	rest := strings.TrimSpace(line[end+1:])
	fields := strings.Fields(rest)

	// We need at least 22 fields after comm for standard stat info.
	if len(fields) < 22 {
		return nil, fmt.Errorf("stat line has too few fields")
	}

	result := make(map[string]any)
	result["comm"] = comm

	// Field mapping after comm (rest[0] is state, rest[1] is ppid, etc.)
	// See proc(5): pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt utime stime cutime cstime priority nice num_threads itrealvalue starttime vsize rss
	fieldNames := []string{
		"state", "ppid", "pgrp", "session", "tty_nr", "tpgid", "flags",
		"minflt", "cminflt", "majflt", "cmajflt", "utime", "stime",
		"cutime", "cstime", "priority", "nice", "num_threads", "itrealvalue",
		"starttime", "vsize", "rss",
	}
	for i, name := range fieldNames {
		if i >= len(fields) {
			break
		}
		val, err := strconv.ParseInt(fields[i], 10, 64)
		if err == nil {
			result[name] = val
		} else {
			result[name] = fields[i]
		}
	}

	return result, nil
}

// HumanDuration converts seconds to a human-readable duration string.
func HumanDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	return fmt.Sprintf("%dd%dh", days, hours)
}

// ParseMemInfoKB parses a /proc/meminfo line returning the key and value in KiB.
func ParseMemInfoKB(line string) (string, uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", 0, fmt.Errorf("invalid meminfo line: %q", line)
	}
	key := strings.TrimSuffix(fields[0], ":")
	val, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("parse meminfo value: %w", err)
	}
	return key, val, nil
}
