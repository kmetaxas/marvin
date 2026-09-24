package linux

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	p := NewProvider(emptyConfig())
	assert.Equal(t, "linux", p.Name())
	assert.True(t, p.IsEnabled(nil))

	// On non-Linux, IsConfigured returns false and capabilities are empty.
	// On Linux, IsConfigured returns true and all 74 capabilities are registered.
	if !p.IsConfigured() {
		assert.Len(t, p.Capabilities(), 0)
		return
	}

	// On Linux: verify all expected capabilities exist.
	assert.Len(t, p.Capabilities(), 74)

	expected := []string{
		"linux.system.info",
		"linux.system.uptime",
		"linux.system.time",
		"linux.system.environment",
		"linux.kernel.cmdline",
		"linux.process.list",
		"linux.memory.info",
		"linux.cpu.info",
		"linux.filesystem.list",
		"linux.blockdevice.list",
		"linux.socket.list",
		"linux.network.interface.list",
		"linux.firewall.nftables.list",
		"linux.firewall.nftables.table.get",
		"linux.firewall.nftables.ruleset",
		"linux.systemd.unit.list",
		"linux.journal.query",
		"linux.user.list",
		"linux.security.capabilities.get",
		"linux.cgroup.list",
		"linux.namespace.list",
		"linux.package.get",
		"linux.core_dump.list",
		"linux.login.sessions",
	}
	for _, name := range expected {
		found, ok := p.GetTask(name)
		require.True(t, ok, "expected task %q to exist", name)
		assert.NotNil(t, found)
		assert.Equal(t, name, found.Name())
	}
}

func TestProviderDefensiveCopy(t *testing.T) {
	t.Parallel()

	p := NewProvider(emptyConfig())
	if !p.IsConfigured() {
		t.Skip("skipping on non-Linux")
	}

	caps := p.Capabilities()
	require.True(t, len(caps) > 0, "expected capabilities on Linux")
	caps[0].Name = "tampered"
	assert.NotEqual(t, "tampered", p.Capabilities()[0].Name)
}

func TestProviderIsConfiguredNonLinux(t *testing.T) {
	t.Parallel()
	p := NewProvider(emptyConfig())
	// On non-Linux: IsConfigured returns false.
	// On Linux: IsConfigured returns true.
	if p.IsConfigured() {
		t.Skip("skipping on Linux")
	}
	assert.False(t, p.IsConfigured())
}

// Compile-time assertions for all tasks.
var _ task.Task = (*systemInfoTask)(nil)
var _ task.Task = (*systemUptimeTask)(nil)
var _ task.Task = (*systemTimeTask)(nil)
var _ task.Task = (*systemEnvironmentTask)(nil)
var _ task.Task = (*kernelCmdlineTask)(nil)
var _ task.Task = (*kernelModulesListTask)(nil)
var _ task.Task = (*kernelTaintTask)(nil)
var _ task.Task = (*kernelSysctlGetTask)(nil)
var _ task.Task = (*kernelDmesgTask)(nil)
var _ task.Task = (*processListTask)(nil)
var _ task.Task = (*processGetTask)(nil)
var _ task.Task = (*processTreeTask)(nil)
var _ task.Task = (*processOpenFilesTask)(nil)
var _ task.Task = (*processLimitsTask)(nil)
var _ task.Task = (*processEnvironmentTask)(nil)
var _ task.Task = (*processThreadsTask)(nil)
var _ task.Task = (*processStackTask)(nil)
var _ task.Task = (*memoryInfoTask)(nil)
var _ task.Task = (*memoryVmstatTask)(nil)
var _ task.Task = (*cpuInfoTask)(nil)
var _ task.Task = (*cpuStatsTask)(nil)
var _ task.Task = (*loadInfoTask)(nil)
var _ task.Task = (*pressureGetTask)(nil)
var _ task.Task = (*swapListTask)(nil)
var _ task.Task = (*filesystemListTask)(nil)
var _ task.Task = (*filesystemInodesTask)(nil)
var _ task.Task = (*filesystemMountsTask)(nil)
var _ task.Task = (*filesystemStatTask)(nil)
var _ task.Task = (*filesystemReadTask)(nil)
var _ task.Task = (*filesystemListDirectoryTask)(nil)
var _ task.Task = (*filesystemSpaceConsumersTask)(nil)
var _ task.Task = (*blockdeviceListTask)(nil)
var _ task.Task = (*blockdeviceStatsTask)(nil)
var _ task.Task = (*socketListTask)(nil)
var _ task.Task = (*socketGetTask)(nil)
var _ task.Task = (*interfaceListTask)(nil)
var _ task.Task = (*routeListTask)(nil)
var _ task.Task = (*routeLookupTask)(nil)
var _ task.Task = (*ruleListTask)(nil)
var _ task.Task = (*neighborListTask)(nil)
var _ task.Task = (*networkStatsTask)(nil)
var _ task.Task = (*dnsConfigTask)(nil)
var _ task.Task = (*dnsResolveTask)(nil)
var _ task.Task = (*nftablesListTask)(nil)
var _ task.Task = (*nftablesTableGetTask)(nil)
var _ task.Task = (*nftablesRulesetTask)(nil)
var _ task.Task = (*iptablesRulesTask)(nil)
var _ task.Task = (*firewalldStatusTask)(nil)
var _ task.Task = (*firewalldZoneGetTask)(nil)
var _ task.Task = (*systemdUnitListTask)(nil)
var _ task.Task = (*systemdUnitStatusTask)(nil)
var _ task.Task = (*systemdUnitShowTask)(nil)
var _ task.Task = (*systemdUnitDefinitionTask)(nil)
var _ task.Task = (*systemdUnitDependenciesTask)(nil)
var _ task.Task = (*systemdUnitLogsTask)(nil)
var _ task.Task = (*journalQueryTask)(nil)
var _ task.Task = (*journalBootsTask)(nil)
var _ task.Task = (*serviceFailedTask)(nil)
var _ task.Task = (*userListTask)(nil)
var _ task.Task = (*groupListTask)(nil)
var _ task.Task = (*userGetTask)(nil)
var _ task.Task = (*capabilitiesGetTask)(nil)
var _ task.Task = (*selinuxStatusTask)(nil)
var _ task.Task = (*apparmorStatusTask)(nil)
var _ task.Task = (*cgroupListTask)(nil)
var _ task.Task = (*cgroupGetTask)(nil)
var _ task.Task = (*cgroupProcessesTask)(nil)
var _ task.Task = (*namespaceListTask)(nil)
var _ task.Task = (*packageGetTask)(nil)
var _ task.Task = (*packageListTask)(nil)
var _ task.Task = (*coreDumpListTask)(nil)
var _ task.Task = (*coreDumpInfoTask)(nil)
var _ task.Task = (*loginSessionsTask)(nil)
var _ task.Task = (*loginHistoryTask)(nil)

func emptyConfig() config.LinuxConfig {
	return config.LinuxConfig{}
}
