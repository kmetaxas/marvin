package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSysBlockSize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "size"), []byte("12345678\n"), 0644))

	size, err := readSysBlockSize(dir)
	require.NoError(t, err)
	assert.Equal(t, uint64(12345678), size)
}

func TestReadProcPartitions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	partitionsFile := filepath.Join(dir, "partitions")
	content := `major minor  #blocks  name

   8        0   10485760 sda
   8        1    1048576 sda1
 259        0    2097152 nvme0n1
`
	require.NoError(t, os.WriteFile(partitionsFile, []byte(content), 0644))

	// Test with a mock by parsing logic
	// Since readProcPartitions reads hardcoded /proc/partitions,
	// we just test parsing.
	major, minor, blocks, err := parseProcPartitionsContent(content, "sda")
	require.NoError(t, err)
	assert.Equal(t, 8, major)
	assert.Equal(t, 0, minor)
	assert.Equal(t, uint64(10485760), blocks)

	_, _, _, err = parseProcPartitionsContent(content, "missing")
	assert.Error(t, err)
}
