//go:build linux

package linux

import (
	"context"
	"testing"

	"github.com/google/nftables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescribeNftablesTableSkipsOtherTables(t *testing.T) {
	t.Parallel()

	table := &nftables.Table{Name: "filter", Family: nftables.TableFamilyINet}
	other := &nftables.Table{Name: "filter", Family: nftables.TableFamilyIPv4}
	got, err := describeNftablesTable(fakeNftablesClient{
		chains: []*nftables.Chain{
			{Name: "input", Table: table},
			{Name: "input", Table: other},
		},
		rules: map[string][]*nftables.Rule{},
	}, table)
	require.NoError(t, err)

	require.Len(t, got.Chains, 1)
	assert.Equal(t, "inet", got.Family)
	assert.Equal(t, "input", got.Chains[0].Name)
}

func TestNftablesTableGetTaskRequiresTableName(t *testing.T) {
	t.Parallel()

	result, err := (&nftablesTableGetTask{}).Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "table_name")
}

func TestNftablesFamilyMappings(t *testing.T) {
	t.Parallel()

	assert.Equal(t, nftables.TableFamilyIPv4, nftablesFamilyFromString("ip"))
	assert.Equal(t, nftables.TableFamilyIPv6, nftablesFamilyFromString("ip6"))
	assert.Equal(t, nftables.TableFamilyINet, nftablesFamilyFromString("inet"))
	assert.Equal(t, nftables.TableFamilyARP, nftablesFamilyFromString("arp"))
	assert.Equal(t, nftables.TableFamilyBridge, nftablesFamilyFromString("bridge"))
	assert.Equal(t, nftables.TableFamilyNetdev, nftablesFamilyFromString("netdev"))
	assert.Equal(t, "ip", nftablesFamilyString(nftables.TableFamilyIPv4))
	assert.Equal(t, "ip6", nftablesFamilyString(nftables.TableFamilyIPv6))
	assert.Equal(t, "inet", nftablesFamilyString(nftables.TableFamilyINet))
	assert.Equal(t, "arp", nftablesFamilyString(nftables.TableFamilyARP))
	assert.Equal(t, "bridge", nftablesFamilyString(nftables.TableFamilyBridge))
	assert.Equal(t, "netdev", nftablesFamilyString(nftables.TableFamilyNetdev))
}
