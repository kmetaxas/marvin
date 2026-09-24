package procfs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Reader is a thin filesystem abstraction for /proc and /sys.
// It is swappable for tests by setting a different Root.
type Reader struct {
	Root string // default "/"
}

// DefaultReader is the production reader pointing at the real filesystem.
var DefaultReader = &Reader{Root: "/"}

// Path joins Root with the given path elements.
func (r *Reader) Path(elem ...string) string {
	return filepath.Join(append([]string{r.Root}, elem...)...)
}

// ReadFileString reads a file and returns its contents as a string.
func (r *Reader) ReadFileString(elem ...string) (string, error) {
	data, err := os.ReadFile(r.Path(elem...))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadFileLines reads a file and returns its contents as a slice of strings.
func (r *Reader) ReadFileLines(elem ...string) ([]string, error) {
	f, err := os.Open(r.Path(elem...))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// ReadDirNames returns the names of entries in a directory.
func (r *Reader) ReadDirNames(elem ...string) ([]string, error) {
	entries, err := os.ReadDir(r.Path(elem...))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// Exists reports whether a file or directory exists.
func (r *Reader) Exists(elem ...string) bool {
	_, err := os.Stat(r.Path(elem...))
	return err == nil
}

// IsDir reports whether the path is a directory.
func (r *Reader) IsDir(elem ...string) bool {
	info, err := os.Stat(r.Path(elem...))
	return err == nil && info.IsDir()
}

// ReadInt reads a single integer from a file.
func (r *Reader) ReadInt(elem ...string) (int, error) {
	s, err := r.ReadFileString(elem...)
	if err != nil {
		return 0, err
	}
	s = strings.TrimSpace(s)
	return strconv.Atoi(s)
}

// ReadUint64 reads a single unsigned integer from a file.
func (r *Reader) ReadUint64(elem ...string) (uint64, error) {
	s, err := r.ReadFileString(elem...)
	if err != nil {
		return 0, err
	}
	s = strings.TrimSpace(s)
	return strconv.ParseUint(s, 10, 64)
}

// Glob returns matching file paths relative to Root.
func (r *Reader) Glob(pattern string) ([]string, error) {
	return filepath.Glob(r.Path(pattern))
}
