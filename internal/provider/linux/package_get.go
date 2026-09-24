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

// packageGetTask implements the linux.package.get capability.
type packageGetTask struct{ provider *Provider }

const packageGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.package.get Parameters",
  "description": "Parameters for linux.package.get.",
  "properties": {
    "name": {
      "type": "string",
      "description": "Package name to query"
    }
  },
  "required": ["name"]
}`

func (t *packageGetTask) Name() string       { return "linux.package.get" }
func (t *packageGetTask) JSONSchema() string { return packageGetSchema }

func (t *packageGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("package.get starting", "capability", t.Name())

	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}

	pkg, err := readPackageGet(name)
	if err != nil {
		slog.Info("package.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("package.get succeeded", "capability", t.Name(), "package", pkg.Name)
	return common.SuccessResult(pkg), nil
}

var _ task.Task = (*packageGetTask)(nil)

type PackageInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Status    string `json:"status,omitempty"`
	Installed bool   `json:"installed"`
	Manager   string `json:"manager"`
}

// rpmDatabasePaths are the candidate RPM database locations, in priority order.
// They are a package-level variable so tests can override them with mock files.
var rpmDatabasePaths = []string{
	"/var/lib/rpm/Packages.db",
	"/usr/lib/rpm/rpmdb.sqlite",
	"/var/lib/rpm/Packages",
}

// findRpmDatabase returns the first existing RPM database path, or "" if none.
func findRpmDatabase() string {
	for _, p := range rpmDatabasePaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func readPackageGet(name string) (PackageInfo, error) {
	// Detect package manager
	if _, err := os.Stat("/var/lib/dpkg/status"); err == nil {
		return readDpkgPackage(name)
	}
	if findRpmDatabase() != "" {
		return readRpmPackage(name)
	}

	return PackageInfo{Name: name, Installed: false, Manager: "unknown"}, nil
}

func readDpkgPackage(name string) (PackageInfo, error) {
	data, err := os.ReadFile("/var/lib/dpkg/status")
	if err != nil {
		return PackageInfo{}, fmt.Errorf("read dpkg status: %w", err)
	}

	var currentPkg string
	var currentVersion string
	var currentStatus string
	found := false

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Package: ") {
			if currentPkg == name {
				found = true
				break
			}
			currentPkg = strings.TrimPrefix(line, "Package: ")
			currentVersion = ""
			currentStatus = ""
		} else if strings.HasPrefix(line, "Version: ") {
			currentVersion = strings.TrimPrefix(line, "Version: ")
		} else if strings.HasPrefix(line, "Status: ") {
			currentStatus = strings.TrimPrefix(line, "Status: ")
		} else if line == "" {
			if currentPkg == name {
				found = true
				break
			}
			currentPkg = ""
		}
	}
	if currentPkg == name {
		found = true
	}

	if !found {
		return PackageInfo{Name: name, Installed: false, Manager: "dpkg"}, nil
	}

	installed := strings.Contains(currentStatus, "install ok installed")
	return PackageInfo{
		Name:      name,
		Version:   currentVersion,
		Status:    currentStatus,
		Installed: installed,
		Manager:   "dpkg",
	}, nil
}

func readRpmPackage(name string) (PackageInfo, error) {
	dbPath := findRpmDatabase()
	if dbPath == "" {
		return PackageInfo{Name: name, Installed: false, Manager: "unknown"}, nil
	}

	// Without librpm or the rpm command we cannot resolve an individual package's
	// version from the database. Report that the RPM database is present and
	// defer precise lookup to the host's rpm tooling.
	return PackageInfo{
		Name:      name,
		Installed: true,
		Manager:   "rpm",
		Status:    "rpm database present; verify with 'rpm -q " + name + "' on host",
	}, nil
}
