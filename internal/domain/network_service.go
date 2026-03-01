package domain

import (
	"context"
	"fmt"
	"net/netip"

	"go4.org/netipx"
)

type networkService struct {
	subnets SubnetRepository
	ips     IPRepository
}

func NewNetworkService(subnets SubnetRepository, ips IPRepository) NetworkService {
	return &networkService{
		subnets: subnets,
		ips:     ips,
	}
}

func (s *networkService) ListSubnets(ctx context.Context) ([]Subnet, error) {
	return s.subnets.List(ctx)
}

func (s *networkService) CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error) {
	if _, err := netip.ParsePrefix(input.CIDR); err != nil {
		return Subnet{}, fmt.Errorf("%w: invalid cidr", ErrInvalidInput)
	}
	return s.subnets.Create(ctx, input)
}

func (s *networkService) GetSubnet(ctx context.Context, id int64) (Subnet, error) {
	return s.subnets.FindByID(ctx, id)
}

func (s *networkService) DeleteSubnet(ctx context.Context, id int64) error {
	deleted, err := s.subnets.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func (s *networkService) ListIPs(ctx context.Context, subnetID int64) ([]IPAddress, error) {
	if _, err := s.subnets.FindByID(ctx, subnetID); err != nil {
		return nil, err
	}
	return s.ips.ListBySubnetID(ctx, subnetID)
}

func (s *networkService) CreateIP(ctx context.Context, subnetID int64, input CreateIPInput) (IPAddress, error) {
	subnet, err := s.subnets.FindByID(ctx, subnetID)
	if err != nil {
		return IPAddress{}, err
	}

	ip, err := netip.ParseAddr(input.IP)
	if err != nil {
		return IPAddress{}, fmt.Errorf("%w: invalid ip", ErrInvalidInput)
	}

	if err = validateIPInSubnet(subnet.CIDR, ip); err != nil {
		return IPAddress{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	return s.ips.Create(ctx, input, subnetID)
}

func (s *networkService) UpdateIPHostname(ctx context.Context, subnetID int64, id IPAddressID, input UpdateIPInput) (IPAddress, error) {
	if _, err := s.subnets.FindByID(ctx, subnetID); err != nil {
		return IPAddress{}, err
	}
	if _, err := s.ips.FindByIDAndSubnet(ctx, id, subnetID); err != nil {
		return IPAddress{}, err
	}
	return s.ips.UpdateHostname(ctx, id, input)
}

func (s *networkService) DeleteIP(ctx context.Context, subnetID int64, id IPAddressID) error {
	deleted, err := s.ips.DeleteByIDAndSubnet(ctx, id, subnetID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func validateIPInSubnet(prefix netip.Prefix, ip netip.Addr) error {
	if !prefix.Contains(ip) {
		return fmt.Errorf("ip not in subnet")
	}

	// /31 IPv4 point-to-point links treat both addresses as usable.
	if ip.Is4() && prefix.Bits() != 31 {
		r := netipx.RangeOfPrefix(prefix)
		if r.From() == ip || r.To() == ip {
			return fmt.Errorf("network or broadcast ip")
		}
	}

	return nil
}
