package domain

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
	"go4.org/netipx"
)

type networkService struct {
	subnets SubnetRepository
	ips     IPRepository
	sites   SiteRepository
}

func NewNetworkService(subnets SubnetRepository, ips IPRepository, sites ...SiteRepository) NetworkService {
	service := &networkService{
		subnets: subnets,
		ips:     ips,
	}
	if len(sites) > 0 {
		service.sites = sites[0]
	}
	return service
}

func (s *networkService) ListSubnets(ctx context.Context) ([]Subnet, error) {
	subnets, err := s.subnets.List(ctx)
	return enrichSubnets(subnets), err
}

func (s *networkService) CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error) {
	if input.SiteID == nil || *input.SiteID == uuid.Nil {
		return Subnet{}, fmt.Errorf("%w: site is required", ErrInvalidInput)
	}
	if err := s.validateSite(ctx, *input.SiteID); err != nil {
		return Subnet{}, err
	}
	cidr, err := netip.ParsePrefix(input.CIDR)
	if err != nil {
		return Subnet{}, fmt.Errorf("%w: invalid cidr", ErrInvalidInput)
	}
	subnet, err := s.subnets.Create(ctx, CreateSubnetRecord{
		CIDR:        cidr,
		SiteID:      input.SiteID,
		Description: input.Description,
	})
	return enrichSubnet(subnet), err
}

func (s *networkService) AssignSubnetSite(ctx context.Context, input AssignSubnetSiteInput) (Subnet, error) {
	if input.SiteID == uuid.Nil {
		return Subnet{}, fmt.Errorf("%w: site is required", ErrInvalidInput)
	}
	if err := s.validateSite(ctx, input.SiteID); err != nil {
		return Subnet{}, err
	}
	subnet, err := s.subnets.AssignSite(ctx, input.ID, input.SiteID)
	if errors.Is(err, ErrNotFound) {
		return Subnet{}, fmt.Errorf("%w: subnet not found", ErrNotFound)
	}
	return enrichSubnet(subnet), err
}

func (s *networkService) GetSubnet(ctx context.Context, id int64) (Subnet, error) {
	subnet, err := s.subnets.FindByID(ctx, id)
	return enrichSubnet(subnet), err
}

func (s *networkService) UpdateSubnet(ctx context.Context, input UpdateSubnetInput) (Subnet, error) {
	if input.SiteID == nil || *input.SiteID == uuid.Nil {
		return Subnet{}, fmt.Errorf("%w: site is required", ErrInvalidInput)
	}
	if err := s.validateSite(ctx, *input.SiteID); err != nil {
		return Subnet{}, err
	}
	cidr, err := netip.ParsePrefix(input.CIDR)
	if err != nil {
		return Subnet{}, fmt.Errorf("%w: invalid cidr", ErrInvalidInput)
	}
	subnet, err := s.subnets.Update(ctx, UpdateSubnetRecord{
		ID:          input.ID,
		CIDR:        cidr,
		SiteID:      input.SiteID,
		Description: input.Description,
	})
	return enrichSubnet(subnet), err
}

func (s *networkService) validateSite(ctx context.Context, siteID uuid.UUID) error {
	if s.sites == nil {
		return nil
	}
	if _, err := s.sites.FindByID(ctx, siteID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("%w: site not found", ErrNotFound)
		}
		return err
	}
	return nil
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

func enrichSubnets(subnets []Subnet) []Subnet {
	for i := range subnets {
		subnets[i] = enrichSubnet(subnets[i])
	}
	return subnets
}

func enrichSubnet(subnet Subnet) Subnet {
	if subnet.CIDR.IsValid() {
		subnet.TotalIPCount = subnetCapacity(subnet.CIDR)
		if subnet.TotalIPCount < subnet.UsedIPCount {
			subnet.TotalIPCount = subnet.UsedIPCount
		}
	}
	return subnet
}

func (s *networkService) ListIPs(ctx context.Context, subnetID int64) ([]IPAddress, error) {
	if _, err := s.subnets.FindByID(ctx, subnetID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrNotFound, ErrSubnetNotFound)
		}
		return nil, err
	}
	return s.ips.ListBySubnetID(ctx, subnetID)
}

func (s *networkService) CreateIP(ctx context.Context, subnetID int64, input CreateIPInput) (IPAddress, error) {
	subnet, err := s.subnets.FindByID(ctx, subnetID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return IPAddress{}, fmt.Errorf("%w: %w", ErrNotFound, ErrSubnetNotFound)
		}
		return IPAddress{}, err
	}

	ip, err := netip.ParseAddr(input.IP)
	if err != nil {
		return IPAddress{}, fmt.Errorf("%w: invalid ip", ErrInvalidInput)
	}

	if err = validateIPInSubnet(subnet.CIDR, ip); err != nil {
		return IPAddress{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	return s.ips.Create(ctx, CreateIPRecord{
		IP:       ip,
		Hostname: input.Hostname,
	}, subnetID)
}

func (s *networkService) UpdateIPHostname(ctx context.Context, subnetID int64, id IPAddressID, input UpdateIPInput) (IPAddress, error) {
	if _, err := s.subnets.FindByID(ctx, subnetID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return IPAddress{}, fmt.Errorf("%w: %w", ErrNotFound, ErrSubnetNotFound)
		}
		return IPAddress{}, err
	}
	if _, err := s.ips.FindByIDAndSubnet(ctx, id, subnetID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return IPAddress{}, fmt.Errorf("%w: %w", ErrNotFound, ErrIPNotFound)
		}
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
