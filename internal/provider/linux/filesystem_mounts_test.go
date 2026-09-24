package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMountinfoLine(t *testing.T) {
	t.Parallel()

	line := "36 35 98:0 /tmp1 /tmp2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue"
	entry, ok := parseMountinfoLine(line)
	require.True(t, ok)

	assert.Equal(t, 36, entry["mount_id"])
	assert.Equal(t, 35, entry["parent_id"])
	assert.Equal(t, 98, entry["device_major"])
	assert.Equal(t, 0, entry["device_minor"])
	assert.Equal(t, "/tmp1", entry["root"])
	assert.Equal(t, "/tmp2", entry["mount_point"])
	assert.Equal(t, []string{"rw", "noatime"}, entry["options"])
	assert.Equal(t, "ext3", entry["filesystem_type"])
	assert.Equal(t, "/dev/root", entry["source"])
	assert.Equal(t, []string{"rw", "errors=continue"}, entry["super_options"])
}

func TestParseMountinfoLineInvalid(t *testing.T) {
	t.Parallel()

	// Too few fields
	_, ok := parseMountinfoLine("1 2 3")
	assert.False(t, ok)

	// Missing separator
	_, ok = parseMountinfoLine("1 2 3:4 / /mnt opt -")
	assert.False(t, ok)
}
