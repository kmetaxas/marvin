package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRpmDatabase(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Packages.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("SQLite format 3\x00"), 0644))

	orig := rpmDatabasePaths
	rpmDatabasePaths = []string{dbPath, filepath.Join(dir, "missing.sqlite")}
	t.Cleanup(func() { rpmDatabasePaths = orig })

	assert.Equal(t, dbPath, findRpmDatabase())
}

func TestFindRpmDatabaseNone(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	orig := rpmDatabasePaths
	rpmDatabasePaths = []string{filepath.Join(dir, "a.db"), filepath.Join(dir, "b.sqlite")}
	t.Cleanup(func() { rpmDatabasePaths = orig })

	assert.Equal(t, "", findRpmDatabase())
}

func TestReadRpmPackageDatabasePresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Packages.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("SQLite format 3\x00"), 0644))

	orig := rpmDatabasePaths
	rpmDatabasePaths = []string{dbPath}
	t.Cleanup(func() { rpmDatabasePaths = orig })

	pkg, err := readRpmPackage("bash")
	require.NoError(t, err)
	assert.Equal(t, "bash", pkg.Name)
	assert.True(t, pkg.Installed)
	assert.Equal(t, "rpm", pkg.Manager)
	assert.Contains(t, pkg.Status, "rpm database present")
}

func TestReadRpmPackageDatabaseAbsent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	orig := rpmDatabasePaths
	rpmDatabasePaths = []string{filepath.Join(dir, "Packages.db")}
	t.Cleanup(func() { rpmDatabasePaths = orig })

	pkg, err := readRpmPackage("bash")
	require.NoError(t, err)
	assert.Equal(t, "bash", pkg.Name)
	assert.False(t, pkg.Installed)
	assert.Equal(t, "unknown", pkg.Manager)
}

func TestExtractRpmPackageNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Packages.db")
	// Simulate SQLite data pages containing package-name-like strings.
	content := "SQLite format 3\x00" +
		"\x00\x01bash\x00" +
		"\x00\x02coreutils\x00" +
		"\x00\x03glibc\x00" +
		"\x00\x04systemd-libs\x00" +
		"\x00\x05garbage!!!\x00" +
		"\x00\x06\x00\x07"
	require.NoError(t, os.WriteFile(dbPath, []byte(content), 0644))

	names := extractRpmPackageNames(dbPath, "", 100)
	assert.Contains(t, names, "bash")
	assert.Contains(t, names, "coreutils")
	assert.Contains(t, names, "glibc")
	assert.Contains(t, names, "systemd-libs")
	assert.NotContains(t, names, "garbage!!!")
}

func TestExtractRpmPackageNamesFilterAndLimit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Packages.db")
	content := "SQLite format 3\x00" +
		"\x00\x01bash\x00" +
		"\x00\x02bash-completion\x00" +
		"\x00\x03coreutils\x00"
	require.NoError(t, os.WriteFile(dbPath, []byte(content), 0644))

	names := extractRpmPackageNames(dbPath, "bash", 100)
	assert.Contains(t, names, "bash")
	assert.Contains(t, names, "bash-completion")
	assert.NotContains(t, names, "coreutils")

	limited := extractRpmPackageNames(dbPath, "", 2)
	assert.Len(t, limited, 2)
}

func TestReadRpmPackageListDatabasePresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Packages.db")
	content := "SQLite format 3\x00" +
		"\x00\x01bash\x00" +
		"\x00\x02coreutils\x00"
	require.NoError(t, os.WriteFile(dbPath, []byte(content), 0644))

	orig := rpmDatabasePaths
	rpmDatabasePaths = []string{dbPath}
	t.Cleanup(func() { rpmDatabasePaths = orig })

	pkgs, err := readRpmPackageList("", 100)
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)
	for _, p := range pkgs {
		assert.Equal(t, "rpm", p.Manager)
		assert.True(t, p.Installed)
	}
}

func TestReadRpmPackageListDatabaseAbsent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	orig := rpmDatabasePaths
	rpmDatabasePaths = []string{filepath.Join(dir, "Packages.db")}
	t.Cleanup(func() { rpmDatabasePaths = orig })

	pkgs, err := readRpmPackageList("", 100)
	require.NoError(t, err)
	assert.Empty(t, pkgs)
}
