package http

import (
	"fmt"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

func parsePathInt64(r *http.Request, name string) (int64, error) {
	v := r.PathValue(name)
	if v == "" {
		return 0, fmt.Errorf("%s missing", name)
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s invalid: %w", name, err)
	}
	return id, nil
}

func strToUUID(name string) (pgtype.UUID, error) {
	u, err := uuid.Parse(name)
	if err != nil {
		return pgtype.UUID{}, err
	}
	var id pgtype.UUID
	copy(id.Bytes[:], u[:])
	id.Valid = true
	return id, nil
}
