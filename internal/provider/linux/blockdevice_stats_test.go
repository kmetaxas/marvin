package linux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadDiskstats(t *testing.T) {
	content := `   8       0 sda 123 456 789 10 11 12 13 14 15 16 17
 259       0 nvme0n1 100 200 300 400 500 600 700 800 900 1000 1100
`
	stats := parseDiskstatsContent(content)
	assert.Len(t, stats, 2)

	sda := stats[0]
	assert.Equal(t, "sda", sda["name"])
	assert.Equal(t, 8, sda["major"])
	assert.Equal(t, 0, sda["minor"])
	assert.Equal(t, uint64(123), sda["reads_completed"])
	assert.Equal(t, uint64(456), sda["reads_merged"])
	assert.Equal(t, uint64(789), sda["sectors_read"])
	assert.Equal(t, uint64(10), sda["ms_reading"])
	assert.Equal(t, uint64(11), sda["writes_completed"])
	assert.Equal(t, uint64(12), sda["writes_merged"])
	assert.Equal(t, uint64(13), sda["sectors_written"])
	assert.Equal(t, uint64(14), sda["ms_writing"])
	assert.Equal(t, uint64(15), sda["ios_in_progress"])
	assert.Equal(t, uint64(16), sda["ms_doing_io"])
	assert.Equal(t, uint64(17), sda["weighted_ms_doing_io"])

	nvme := stats[1]
	assert.Equal(t, "nvme0n1", nvme["name"])
	assert.Equal(t, uint64(100), nvme["reads_completed"])
}
