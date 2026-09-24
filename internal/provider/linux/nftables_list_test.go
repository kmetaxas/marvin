//go:build linux

package linux

import (
	"context"
	"errors"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNftablesClient struct {
	tables []*nftables.Table
	chains []*nftables.Chain
	rules  map[string][]*nftables.Rule
	err    error
}

func (f fakeNftablesClient) ListTables() ([]*nftables.Table, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tables, nil
}

func (f fakeNftablesClient) ListTableOfFamily(name string, family nftables.TableFamily) (*nftables.Table, error) {
	for _, table := range f.tables {
		if table.Name == name && table.Family == family {
			return table, nil
		}
	}
	return nil, nil
}

func (f fakeNftablesClient) ListChains() ([]*nftables.Chain, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.chains, nil
}

func (f fakeNftablesClient) GetRules(t *nftables.Table, c *nftables.Chain) ([]*nftables.Rule, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rules[t.Name+"/"+c.Name], nil
}

func TestListNftablesTablesReturnsStructuredTablesChainsAndRules(t *testing.T) {
	t.Parallel()

	table := &nftables.Table{Name: "filter", Family: nftables.TableFamilyINet, Use: 1}
	policy := nftables.ChainPolicyDrop
	priority := nftables.ChainPriority(0)
	chain := &nftables.Chain{
		Name:     "input",
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookInput,
		Priority: &priority,
		Policy:   &policy,
	}
	rule := &nftables.Rule{Table: table, Chain: chain, Handle: 42, Exprs: []expr.Any{&expr.Counter{Packets: 7, Bytes: 99}}}

	got, err := listNftablesTables(fakeNftablesClient{
		tables: []*nftables.Table{table, {Name: "nat", Family: nftables.TableFamilyIPv4}},
		chains: []*nftables.Chain{chain},
		rules:  map[string][]*nftables.Rule{"filter/input": {rule}},
	}, "filter", "inet")
	require.NoError(t, err)
	require.Len(t, got, 1)

	assert.Equal(t, "filter", got[0].Name)
	assert.Equal(t, "inet", got[0].Family)
	assert.Equal(t, uint32(1), got[0].Use)
	assert.Equal(t, []nftablesChain{{Name: "input", Type: "filter", Hook: "input", Policy: "drop"}}, got[0].Chains)
	require.Len(t, got[0].Rules, 1)
	assert.Equal(t, uint64(42), got[0].Rules[0].Handle)
	assert.Equal(t, "input", got[0].Rules[0].Chain)
	assert.Contains(t, got[0].Rules[0].Expressions, "*expr.Counter")
}

func TestListNftablesTablesRejectsInvalidFamily(t *testing.T) {
	t.Parallel()

	_, err := listNftablesTables(fakeNftablesClient{}, "", "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "family")
}

func TestListNftablesTablesWrapsClientErrors(t *testing.T) {
	t.Parallel()

	_, err := listNftablesTables(fakeNftablesClient{err: errors.New("denied")}, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list nftables tables")
}

func TestNftablesListTaskRejectsBadParameterTypes(t *testing.T) {
	t.Parallel()

	result, err := (&nftablesListTask{}).Execute(context.Background(), map[string]any{"family": 123})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "family")
}
