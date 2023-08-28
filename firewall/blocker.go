package firewall

import "net"

type Blocker interface {
	BlockIP(address net.IP) error
	BlockIPList(blacklist []net.IP) error
	UnblockIP(address net.IP) error
	ResetFirewallRules() error
}
