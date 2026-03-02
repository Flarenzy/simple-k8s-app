package domain

import (
	"net/netip"
	"testing"
)

func FuzzValidateIPInSubnet(f *testing.F) {
	f.Add("10.0.0.0/24", "10.0.0.10")
	f.Add("10.0.0.0/24", "10.0.1.10")
	f.Add("10.0.0.0/31", "10.0.0.1")
	f.Add("bad-prefix", "10.0.0.1")
	f.Add("10.0.0.0/24", "bad-ip")

	f.Fuzz(func(t *testing.T, prefixStr string, ipStr string) {
		prefix, err := netip.ParsePrefix(prefixStr)
		if err != nil {
			t.Skip()
		}

		ip, err := netip.ParseAddr(ipStr)
		if err != nil {
			t.Skip()
		}

		_ = validateIPInSubnet(prefix, ip)
	})
}
