package http

import (
	"fmt"
	"net/netip"

	"go4.org/netipx"
)

func validateIPInSubnet(p netip.Prefix, ip netip.Addr) error {
	if !p.Contains(ip) {
		return fmt.Errorf("ip not in subnet")
	}

	if ip.Is4() && p.Bits() != 31 { // special case /31 point to point links as those are tehnically both broadcast and network
		r := netipx.RangeOfPrefix(p)
		if r.From() == ip || r.To() == ip {
			return fmt.Errorf("network or broadcast ip")
		}
	}
	return nil
}
