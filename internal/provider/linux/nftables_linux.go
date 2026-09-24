//go:build linux

package linux

import (
	"fmt"
	"strings"

	"github.com/google/nftables"
)

type nftablesClient interface {
	ListTables() ([]*nftables.Table, error)
	ListTableOfFamily(name string, family nftables.TableFamily) (*nftables.Table, error)
	ListChains() ([]*nftables.Chain, error)
	GetRules(t *nftables.Table, c *nftables.Chain) ([]*nftables.Rule, error)
}

func readNftablesRulesetFromNetlink() (map[string]any, error) {
	c, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("open nftables netlink connection: %w", err)
	}
	tables, err := listNftablesTables(c, "", "")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"source": "netlink",
		"tables": tables,
	}, nil
}

func listNftablesTablesFromKernel(tableName, family string) ([]nftablesTable, error) {
	c, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("open nftables netlink connection: %w", err)
	}
	return listNftablesTables(c, tableName, family)
}

func getNftablesTableFromKernel(tableName, family string) (nftablesTable, error) {
	if err := validateNftablesFamily(family); err != nil {
		return nftablesTable{}, err
	}
	c, err := nftables.New()
	if err != nil {
		return nftablesTable{}, fmt.Errorf("open nftables netlink connection: %w", err)
	}
	table, err := c.ListTableOfFamily(tableName, nftablesFamilyFromString(family))
	if err != nil {
		return nftablesTable{}, fmt.Errorf("list nftables table %q family %q: %w", tableName, family, err)
	}
	if table == nil {
		return nftablesTable{}, fmt.Errorf("nftables table %q family %q not found", tableName, family)
	}
	return describeNftablesTable(c, table)
}

func listNftablesTables(c nftablesClient, tableName, family string) ([]nftablesTable, error) {
	if family != "" {
		if err := validateNftablesFamily(family); err != nil {
			return nil, err
		}
	}

	rawTables, err := c.ListTables()
	if err != nil {
		return nil, fmt.Errorf("list nftables tables: %w", err)
	}

	tables := make([]nftablesTable, 0, len(rawTables))
	for _, table := range rawTables {
		if table == nil {
			continue
		}
		if tableName != "" && table.Name != tableName {
			continue
		}
		if family != "" && nftablesFamilyString(table.Family) != family {
			continue
		}
		described, err := describeNftablesTable(c, table)
		if err != nil {
			return nil, err
		}
		tables = append(tables, described)
	}
	return tables, nil
}

func describeNftablesTable(c nftablesClient, table *nftables.Table) (nftablesTable, error) {
	chains, err := c.ListChains()
	if err != nil {
		return nftablesTable{}, fmt.Errorf("list nftables chains: %w", err)
	}

	described := nftablesTable{
		Name:   table.Name,
		Family: nftablesFamilyString(table.Family),
		Use:    table.Use,
		Flags:  table.Flags,
	}

	for _, chain := range chains {
		if chain == nil || !sameNftablesTable(table, chain.Table) {
			continue
		}
		described.Chains = append(described.Chains, nftablesChainFromKernel(chain))

		rules, err := c.GetRules(table, chain)
		if err != nil {
			return nftablesTable{}, fmt.Errorf("list nftables rules for %s/%s: %w", table.Name, chain.Name, err)
		}
		for _, rule := range rules {
			if rule == nil {
				continue
			}
			described.Rules = append(described.Rules, nftablesRuleFromKernel(rule, chain.Name))
		}
	}

	return described, nil
}

func sameNftablesTable(a, b *nftables.Table) bool {
	return a != nil && b != nil && a.Name == b.Name && a.Family == b.Family
}

func nftablesChainFromKernel(chain *nftables.Chain) nftablesChain {
	described := nftablesChain{
		Name:   chain.Name,
		Type:   string(chain.Type),
		Device: chain.Device,
	}
	if chain.Hooknum != nil {
		described.Hook = nftablesHookString(*chain.Hooknum)
	}
	if chain.Priority != nil {
		described.Priority = int(*chain.Priority)
	}
	if chain.Policy != nil {
		described.Policy = nftablesPolicyString(*chain.Policy)
	}
	return described
}

func nftablesRuleFromKernel(rule *nftables.Rule, chainName string) nftablesRule {
	if chainName == "" && rule.Chain != nil {
		chainName = rule.Chain.Name
	}
	exprs := make([]string, 0, len(rule.Exprs))
	for _, expression := range rule.Exprs {
		exprs = append(exprs, fmt.Sprintf("%T: %v", expression, expression))
	}
	return nftablesRule{
		Handle:      rule.Handle,
		Chain:       chainName,
		Expressions: strings.Join(exprs, "; "),
	}
}

func nftablesFamilyFromString(family string) nftables.TableFamily {
	switch family {
	case "ip":
		return nftables.TableFamilyIPv4
	case "ip6":
		return nftables.TableFamilyIPv6
	case "inet":
		return nftables.TableFamilyINet
	case "arp":
		return nftables.TableFamilyARP
	case "bridge":
		return nftables.TableFamilyBridge
	case "netdev":
		return nftables.TableFamilyNetdev
	default:
		return nftables.TableFamilyUnspecified
	}
}

func nftablesFamilyString(family nftables.TableFamily) string {
	switch family {
	case nftables.TableFamilyIPv4:
		return "ip"
	case nftables.TableFamilyIPv6:
		return "ip6"
	case nftables.TableFamilyINet:
		return "inet"
	case nftables.TableFamilyARP:
		return "arp"
	case nftables.TableFamilyBridge:
		return "bridge"
	case nftables.TableFamilyNetdev:
		return "netdev"
	default:
		return "unspecified"
	}
}

func nftablesHookString(hook nftables.ChainHook) string {
	switch hook {
	case *nftables.ChainHookPrerouting:
		return "prerouting"
	case *nftables.ChainHookInput:
		return "input"
	case *nftables.ChainHookForward:
		return "forward"
	case *nftables.ChainHookOutput:
		return "output"
	case *nftables.ChainHookPostrouting:
		return "postrouting"
	case *nftables.ChainHookIngress:
		return "ingress"
	case *nftables.ChainHookEgress:
		return "egress"
	default:
		return fmt.Sprintf("unknown(%d)", hook)
	}
}

func nftablesPolicyString(policy nftables.ChainPolicy) string {
	switch policy {
	case nftables.ChainPolicyAccept:
		return "accept"
	case nftables.ChainPolicyDrop:
		return "drop"
	default:
		return fmt.Sprintf("unknown(%d)", policy)
	}
}
