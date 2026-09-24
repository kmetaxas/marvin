//go:build !linux

package linux

import "fmt"

func readNftablesRulesetFromNetlink() (map[string]any, error) {
	return nil, fmt.Errorf("nftables netlink is only available on linux")
}

func listNftablesTablesFromKernel(tableName, family string) ([]nftablesTable, error) {
	return nil, fmt.Errorf("nftables netlink is only available on linux")
}

func getNftablesTableFromKernel(tableName, family string) (nftablesTable, error) {
	return nftablesTable{}, fmt.Errorf("nftables netlink is only available on linux")
}
