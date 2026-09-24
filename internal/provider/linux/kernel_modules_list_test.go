package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadKernelModulesList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0755))

	modules := `ext4 708608 1 - Live 0x0000000000000000
ext3 73728 0 - Live 0x0000000000000000
`
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "modules"), []byte(modules), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelModulesList(r)
	require.NoError(t, err)

	mods, ok := data["modules"].([]moduleInfo)
	require.True(t, ok)
	require.Len(t, mods, 2)

	assert.Equal(t, "ext4", mods[0].Name)
	assert.Equal(t, int64(708608), mods[0].Size)
	assert.Equal(t, 1, mods[0].Instances)
	assert.Empty(t, mods[0].Dependencies)
	assert.Equal(t, "Live", mods[0].State)

	assert.Equal(t, "ext3", mods[1].Name)
	assert.Equal(t, 0, mods[1].Instances)
	assert.Empty(t, mods[1].Dependencies)
	assert.Equal(t, 2, data["count"])
}

func TestReadKernelModulesListWithDeps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	procDir := filepath.Join(dir, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0755))

	modules := `crypto_aes 16384 1 crypto_algapi, Live 0x0000000000000000
`
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "modules"), []byte(modules), 0644))

	r := &procfs.Reader{Root: dir}
	data, err := readKernelModulesList(r)
	require.NoError(t, err)

	mods, ok := data["modules"].([]moduleInfo)
	require.True(t, ok)
	require.Len(t, mods, 1)

	assert.Equal(t, []string{"crypto_algapi"}, mods[0].Dependencies)
}
