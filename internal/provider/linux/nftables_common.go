package linux

import "fmt"

type nftablesTable struct {
	Name     string          `json:"name"`
	Family   string          `json:"family"`
	Chains   []nftablesChain `json:"chains,omitempty"`
	Rules    []nftablesRule  `json:"rules,omitempty"`
	Use      uint32          `json:"use,omitempty"`
	Flags    uint32          `json:"flags,omitempty"`
	Messages []string        `json:"messages,omitempty"`
}

type nftablesChain struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Hook     string `json:"hook,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Policy   string `json:"policy,omitempty"`
	Device   string `json:"device,omitempty"`
}

type nftablesRule struct {
	Handle      uint64 `json:"handle,omitempty"`
	Chain       string `json:"chain,omitempty"`
	Expressions string `json:"expressions,omitempty"`
}

func validateNftablesFamily(family string) error {
	switch family {
	case "ip", "ip6", "inet", "arp", "bridge", "netdev":
		return nil
	default:
		return fmt.Errorf("parameter family must be one of ip, ip6, inet, arp, bridge, netdev")
	}
}
