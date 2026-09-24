package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadNftablesRulesetFromProcfs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "net"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "net", "nf_tables"), []byte("filter inet\n# ignored\nnat ip\n"), 0644))

	got, err := readNftablesRulesetFromProcfs(&procfs.Reader{Root: dir})
	require.NoError(t, err)

	assert.Equal(t, "procfs", got["source"])
	assert.Equal(t, []map[string]any{{"name": "filter", "family": "inet"}, {"name": "nat", "family": "ip"}}, got["tables"])
	assert.Empty(t, got["messages"])
}
