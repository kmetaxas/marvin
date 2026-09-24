package linux

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadMemoryInfo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "meminfo"), []byte(`
MemTotal:       65573076 kB
MemFree:         5962396 kB
MemAvailable:   48390260 kB
SwapTotal:             0 kB
SwapFree:              0 kB
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readMemoryInfo(r)
	require.NoError(t, err)
	assert.Equal(t, uint64(65573076), result["MemTotal"])
	assert.Equal(t, uint64(5962396), result["MemFree"])
	assert.Equal(t, uint64(48390260), result["MemAvailable"])
	assert.Equal(t, uint64(65573076*1024), result["mem_total_bytes"])
	assert.Equal(t, uint64(5962396*1024), result["mem_free_bytes"])
	assert.Equal(t, uint64(48390260*1024), result["mem_available_bytes"])
	assert.Equal(t, uint64(0), result["swap_total_bytes"])
	assert.Equal(t, uint64(0), result["swap_free_bytes"])
}

func TestReadMemoryInfoMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	_, err := readMemoryInfo(r)
	require.Error(t, err)
}

func TestReadMemoryVmstat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "vmstat"), []byte(`nr_free_pages 1491083
nr_zone_inactive_anon 38485
nr_zone_active_anon 3460319
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readMemoryVmstat(r)
	require.NoError(t, err)
	assert.Equal(t, uint64(1491083), result["nr_free_pages"])
	assert.Equal(t, uint64(38485), result["nr_zone_inactive_anon"])
	assert.Equal(t, uint64(3460319), result["nr_zone_active_anon"])
}

func TestReadMemoryVmstatMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	_, err := readMemoryVmstat(r)
	require.Error(t, err)
}

func TestReadCPUInfo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "cpuinfo"), []byte(`processor	: 0
model name	: Test CPU
vendor_id	: GenuineTest
cpu cores	: 8
siblings	: 16
cpu MHz		: 2400.0
flags		: fpu sse aes
physical id	: 0
core id		: 0

processor	: 1
model name	: Test CPU
vendor_id	: GenuineTest
cpu cores	: 8
siblings	: 16
cpu MHz		: 2400.0
flags		: fpu sse aes
physical id	: 0
core id		: 1
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readCPUInfo(r)
	require.NoError(t, err)
	assert.Equal(t, 2, result["count"])
	assert.Equal(t, 1, result["physical_cpus"])

	cpus, ok := result["processors"].([]CPUInfo)
	require.True(t, ok)
	require.Len(t, cpus, 2)
	assert.Equal(t, 0, cpus[0].Processor)
	assert.Equal(t, "Test CPU", cpus[0].ModelName)
	assert.Equal(t, "GenuineTest", cpus[0].VendorID)
	assert.Equal(t, 8, cpus[0].CPUCores)
	assert.Equal(t, 16, cpus[0].Siblings)
	assert.InDelta(t, 2400.0, cpus[0].CPUMHz, 0.01)
	assert.Equal(t, []string{"fpu", "sse", "aes"}, cpus[0].Flags)
	assert.Equal(t, 0, cpus[0].PhysicalID)
	assert.Equal(t, 0, cpus[0].CoreID)
	assert.Equal(t, 1, cpus[1].Processor)
	assert.Equal(t, 1, cpus[1].CoreID)
}

func TestReadCPUInfoMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	_, err := readCPUInfo(r)
	require.Error(t, err)
}

func TestReadCPUStats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "stat"), []byte(`cpu  38799625 9803 18328969 4162187781 3015371 4253108 1733385 0 0 0
cpu0 1019865 110 5045565 124498031 62992 821524 431809 0 0 0
cpu1 141503 34 45874 131887584 13378 12842 151759 0 0 0
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readCPUStats(r, false)
	require.NoError(t, err)
	total, ok := result["total"].(*CPUStatLine)
	require.True(t, ok)
	assert.Equal(t, "cpu", total.Name)
	assert.Equal(t, uint64(38799625), total.User)
	assert.Equal(t, uint64(9803), total.Nice)
	assert.Equal(t, uint64(18328969), total.System)
	assert.Equal(t, uint64(4162187781), total.Idle)
	assert.Equal(t, uint64(3015371), total.IOWait)
	assert.Equal(t, uint64(4253108), total.IRQ)
	assert.Equal(t, uint64(1733385), total.SoftIRQ)
	assert.Equal(t, uint64(0), total.Steal)
	assert.Equal(t, uint64(0), total.Guest)
	assert.Equal(t, uint64(0), total.GuestNice)
	_, hasPerCPU := result["per_cpu"]
	assert.False(t, hasPerCPU)
}

func TestReadCPUStatsPerCPU(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "stat"), []byte(`cpu  38799625 9803 18328969 4162187781 3015371 4253108 1733385 0 0 0
cpu0 1019865 110 5045565 124498031 62992 821524 431809 0 0 0
cpu1 141503 34 45874 131887584 13378 12842 151759 0 0 0
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readCPUStats(r, true)
	require.NoError(t, err)
	perCPUs, ok := result["per_cpu"].([]CPUStatLine)
	require.True(t, ok)
	require.Len(t, perCPUs, 2)
	assert.Equal(t, "cpu0", perCPUs[0].Name)
	assert.Equal(t, uint64(1019865), perCPUs[0].User)
	assert.Equal(t, "cpu1", perCPUs[1].Name)
	assert.Equal(t, uint64(141503), perCPUs[1].User)
}

func TestReadCPUStatsMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	_, err := readCPUStats(r, false)
	require.Error(t, err)
}

func TestReadLoadInfo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "loadavg"), []byte(`1.98 2.16 2.37 2/2957 86161
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readLoadInfo(r)
	require.NoError(t, err)
	assert.InDelta(t, 1.98, result["loadavg_1min"], 0.01)
	assert.InDelta(t, 2.16, result["loadavg_5min"], 0.01)
	assert.InDelta(t, 2.37, result["loadavg_15min"], 0.01)
	assert.Equal(t, 2, result["runnable_tasks"])
	assert.Equal(t, 2957, result["total_tasks"])
	assert.Equal(t, 86161, result["last_pid"])
}

func TestReadLoadInfoMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	_, err := readLoadInfo(r)
	require.Error(t, err)
}

func TestReadPressure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "proc", "pressure"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "pressure", "cpu"), []byte(`some avg10=1.23 avg60=4.56 avg300=7.89 total=123456789
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "pressure", "memory"), []byte(`some avg10=0.12 avg60=0.34 avg300=0.56 total=9876543210
full avg10=0.01 avg60=0.02 avg300=0.03 total=111222333
`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "pressure", "io"), []byte(`some avg10=5.00 avg60=6.00 avg300=7.00 total=444555666
full avg10=1.00 avg60=2.00 avg300=3.00 total=777888999
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readPressure(r)
	require.NoError(t, err)

	cpu, ok := result["cpu"].(*PressureResource)
	require.True(t, ok)
	require.NotNil(t, cpu.Some)
	assert.InDelta(t, 1.23, cpu.Some.Avg10, 0.01)
	assert.InDelta(t, 4.56, cpu.Some.Avg60, 0.01)
	assert.InDelta(t, 7.89, cpu.Some.Avg300, 0.01)
	assert.Equal(t, uint64(123456789), cpu.Some.Total)
	assert.Nil(t, cpu.Full)

	mem, ok := result["memory"].(*PressureResource)
	require.True(t, ok)
	require.NotNil(t, mem.Some)
	assert.InDelta(t, 0.12, mem.Some.Avg10, 0.01)
	assert.Equal(t, uint64(9876543210), mem.Some.Total)
	require.NotNil(t, mem.Full)
	assert.InDelta(t, 0.01, mem.Full.Avg10, 0.01)
	assert.Equal(t, uint64(111222333), mem.Full.Total)

	ioRes, ok := result["io"].(*PressureResource)
	require.True(t, ok)
	require.NotNil(t, ioRes.Some)
	assert.InDelta(t, 5.00, ioRes.Some.Avg10, 0.01)
	require.NotNil(t, ioRes.Full)
	assert.InDelta(t, 1.00, ioRes.Full.Avg10, 0.01)
}

func TestReadPressureMissingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	result, err := readPressure(r)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestReadSwapList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "swaps"), []byte(`Filename			Type		Size		Used		Priority
/swapfile                               file		8388604		0		-2
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readSwapList(r)
	require.NoError(t, err)
	assert.Equal(t, 1, result["count"])

	entries, ok := result["swaps"].([]SwapEntry)
	require.True(t, ok)
	require.Len(t, entries, 1)
	assert.Equal(t, "/swapfile", entries[0].Filename)
	assert.Equal(t, "file", entries[0].Type)
	assert.Equal(t, uint64(8388604), entries[0].Size)
	assert.Equal(t, uint64(0), entries[0].Used)
	assert.Equal(t, -2, entries[0].Priority)
}

func TestReadSwapListEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "proc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "proc", "swaps"), []byte(`Filename			Type		Size		Used		Priority
`), 0644))

	r := &procfs.Reader{Root: dir}
	result, err := readSwapList(r)
	require.NoError(t, err)
	assert.Equal(t, 0, result["count"])
}

func TestReadSwapListMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &procfs.Reader{Root: dir}
	_, err := readSwapList(r)
	require.Error(t, err)
}
