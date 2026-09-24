package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilesystemInodesParseMounts(t *testing.T) {
	content := `/dev/sda1 / ext4 rw,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
`
	entries := parseMountsContent(content)
	assert.Len(t, entries, 2)
	assert.Equal(t, "/", entries[0].Target)
	assert.Equal(t, "/proc", entries[1].Target)
}
