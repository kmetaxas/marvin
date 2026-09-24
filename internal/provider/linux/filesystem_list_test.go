package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadMounts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mountsFile := filepath.Join(dir, "mounts")
	content := `/dev/sda1 / ext4 rw,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
`
	require.NoError(t, os.WriteFile(mountsFile, []byte(content), 0644))

	// Temporarily override the file read in readMounts
	origMounts := "/proc/mounts"
	_ = origMounts
	// Since readMounts hardcodes /proc/mounts, we test parsing indirectly.
	entries := parseMountsContent(content)
	assert.Len(t, entries, 2)
	assert.Equal(t, "/dev/sda1", entries[0].Source)
	assert.Equal(t, "/", entries[0].Target)
	assert.Equal(t, "ext4", entries[0].FilesystemType)
	assert.Equal(t, []string{"rw", "relatime"}, entries[0].Options)
	assert.Equal(t, "proc", entries[1].Source)
}
