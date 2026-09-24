package linux

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// packageListTask implements the linux.package.list capability.
type packageListTask struct{ provider *Provider }

const packageListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.package.list Parameters",
  "description": "Parameters for linux.package.list.",
  "properties": {
    "filter": {
      "type": "string",
      "description": "Substring filter on package name"
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of results",
      "default": 100
    }
  }
}`

func (t *packageListTask) Name() string       { return "linux.package.list" }
func (t *packageListTask) JSONSchema() string { return packageListSchema }

func (t *packageListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("package.list starting", "capability", t.Name())

	filter, _ := common.OptionalString(params, "filter", "")
	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}

	pkgs, err := readPackageList(filter, limit)
	if err != nil {
		slog.Info("package.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("package.list succeeded", "capability", t.Name(), "count", len(pkgs))
	return common.SuccessResult(map[string]any{"packages": pkgs}), nil
}

var _ task.Task = (*packageListTask)(nil)

func readPackageList(filter string, limit int) ([]PackageInfo, error) {
	// Detect package manager
	if _, err := os.Stat("/var/lib/dpkg/status"); err == nil {
		return readDpkgPackageList(filter, limit)
	}
	if findRpmDatabase() != "" {
		return readRpmPackageList(filter, limit)
	}

	return []PackageInfo{}, nil
}

func readDpkgPackageList(filter string, limit int) ([]PackageInfo, error) {
	data, err := os.ReadFile("/var/lib/dpkg/status")
	if err != nil {
		return nil, fmt.Errorf("read dpkg status: %w", err)
	}

	var packages []PackageInfo
	var currentPkg, currentVersion, currentStatus string

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Package: ") {
			if currentPkg != "" {
				if filter == "" || strings.Contains(currentPkg, filter) {
					installed := strings.Contains(currentStatus, "install ok installed")
					packages = append(packages, PackageInfo{
						Name:      currentPkg,
						Version:   currentVersion,
						Status:    currentStatus,
						Installed: installed,
						Manager:   "dpkg",
					})
					if len(packages) >= limit {
						break
					}
				}
			}
			currentPkg = strings.TrimPrefix(line, "Package: ")
			currentVersion = ""
			currentStatus = ""
		} else if strings.HasPrefix(line, "Version: ") {
			currentVersion = strings.TrimPrefix(line, "Version: ")
		} else if strings.HasPrefix(line, "Status: ") {
			currentStatus = strings.TrimPrefix(line, "Status: ")
		}
	}
	if currentPkg != "" {
		if filter == "" || strings.Contains(currentPkg, filter) {
			installed := strings.Contains(currentStatus, "install ok installed")
			packages = append(packages, PackageInfo{
				Name:      currentPkg,
				Version:   currentVersion,
				Status:    currentStatus,
				Installed: installed,
				Manager:   "dpkg",
			})
		}
	}
	return packages, nil
}

func readRpmPackageList(filter string, limit int) ([]PackageInfo, error) {
	dbPath := findRpmDatabase()
	if dbPath == "" {
		return []PackageInfo{}, nil
	}

	names := extractRpmPackageNames(dbPath, filter, limit)
	if len(names) == 0 {
		return []PackageInfo{
			{
				Name:    "rpm",
				Manager: "rpm",
				Status:  "rpm database present; package names could not be extracted without librpm",
			},
		}, nil
	}

	packages := make([]PackageInfo, 0, len(names))
	for _, n := range names {
		packages = append(packages, PackageInfo{
			Name:      n,
			Installed: true,
			Manager:   "rpm",
			Status:    "rpm database present; version requires 'rpm -q " + n + "' on host",
		})
	}
	return packages, nil
}

// extractRpmPackageNames scans an RPM database file for printable strings that
// resemble package names. SQLite stores string values inline in its data pages,
// so a heuristic scan recovers a useful subset of names without a SQLite parser.
func extractRpmPackageNames(path, filter string, limit int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var names []string

	var b strings.Builder
	flush := func() {
		s := b.String()
		b.Reset()
		if len(s) < 3 || len(s) > 64 {
			return
		}
		if !isPlausiblePackageName(s) {
			return
		}
		if filter != "" && !strings.Contains(s, filter) {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		names = append(names, s)
	}

	for _, c := range data {
		if c >= 0x20 && c < 0x7f {
			b.WriteByte(c)
			continue
		}
		flush()
		if len(names) >= limit {
			return names
		}
	}
	flush()

	if len(names) > limit {
		names = names[:limit]
	}
	return names
}

// isPlausiblePackageName reports whether s looks like an RPM package name:
// lowercase letters, digits, and the separators '-', '_', '+', '.'.
func isPlausiblePackageName(s string) bool {
	hasLetter := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			hasLetter = true
		case c >= 'A' && c <= 'Z':
			hasLetter = true
		case c >= '0' && c <= '9':
		case c == '-' || c == '_' || c == '+' || c == '.':
		default:
			return false
		}
	}
	return hasLetter
}
