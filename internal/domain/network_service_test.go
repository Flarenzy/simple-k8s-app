package domain

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

type stubSubnetRepository struct {
	listFn   func(context.Context) ([]Subnet, error)
	findFn   func(context.Context, int64) (Subnet, error)
	createFn func(context.Context, CreateSubnetInput) (Subnet, error)
	deleteFn func(context.Context, int64) (bool, error)
}

func (s stubSubnetRepository) List(ctx context.Context) ([]Subnet, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx)
}

func (s stubSubnetRepository) FindByID(ctx context.Context, id int64) (Subnet, error) {
	if s.findFn == nil {
		return Subnet{}, nil
	}
	return s.findFn(ctx, id)
}

func (s stubSubnetRepository) Create(ctx context.Context, input CreateSubnetInput) (Subnet, error) {
	if s.createFn == nil {
		return Subnet{}, nil
	}
	return s.createFn(ctx, input)
}

func (s stubSubnetRepository) Delete(ctx context.Context, id int64) (bool, error) {
	if s.deleteFn == nil {
		return false, nil
	}
	return s.deleteFn(ctx, id)
}

type stubIPRepository struct {
	listFn   func(context.Context, int64) ([]IPAddress, error)
	findFn   func(context.Context, IPAddressID, int64) (IPAddress, error)
	createFn func(context.Context, CreateIPInput, int64) (IPAddress, error)
	updateFn func(context.Context, IPAddressID, UpdateIPInput) (IPAddress, error)
	deleteFn func(context.Context, IPAddressID, int64) (bool, error)
}

func (s stubIPRepository) ListBySubnetID(ctx context.Context, subnetID int64) ([]IPAddress, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, subnetID)
}

func (s stubIPRepository) FindByIDAndSubnet(ctx context.Context, id IPAddressID, subnetID int64) (IPAddress, error) {
	if s.findFn == nil {
		return IPAddress{}, nil
	}
	return s.findFn(ctx, id, subnetID)
}

func (s stubIPRepository) Create(ctx context.Context, input CreateIPInput, subnetID int64) (IPAddress, error) {
	if s.createFn == nil {
		return IPAddress{}, nil
	}
	return s.createFn(ctx, input, subnetID)
}

func (s stubIPRepository) UpdateHostname(ctx context.Context, id IPAddressID, input UpdateIPInput) (IPAddress, error) {
	if s.updateFn == nil {
		return IPAddress{}, nil
	}
	return s.updateFn(ctx, id, input)
}

func (s stubIPRepository) DeleteByIDAndSubnet(ctx context.Context, id IPAddressID, subnetID int64) (bool, error) {
	if s.deleteFn == nil {
		return false, nil
	}
	return s.deleteFn(ctx, id, subnetID)
}

func TestCreateSubnetRejectsInvalidCIDR(t *testing.T) {
	svc := NewNetworkService(stubSubnetRepository{}, stubIPRepository{})

	_, err := svc.CreateSubnet(context.Background(), CreateSubnetInput{CIDR: "not-a-cidr"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateIPRejectsIPOutsideSubnet(t *testing.T) {
	svc := NewNetworkService(
		stubSubnetRepository{
			findFn: func(context.Context, int64) (Subnet, error) {
				prefix, _ := netip.ParsePrefix("10.0.0.0/24")
				return Subnet{ID: 1, CIDR: prefix}, nil
			},
		},
		stubIPRepository{},
	)

	_, err := svc.CreateIP(context.Background(), 1, CreateIPInput{IP: "10.0.1.10"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateIPAllowsUsableAddress(t *testing.T) {
	created := false
	svc := NewNetworkService(
		stubSubnetRepository{
			findFn: func(context.Context, int64) (Subnet, error) {
				prefix, _ := netip.ParsePrefix("10.0.0.0/24")
				return Subnet{ID: 1, CIDR: prefix}, nil
			},
		},
		stubIPRepository{
			createFn: func(_ context.Context, input CreateIPInput, subnetID int64) (IPAddress, error) {
				created = true
				ip, _ := netip.ParseAddr(input.IP)
				return IPAddress{ID: IPAddressID("ip-1"), IP: ip, SubnetID: subnetID}, nil
			},
		},
	)

	ip, err := svc.CreateIP(context.Background(), 1, CreateIPInput{IP: "10.0.0.10"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !created {
		t.Fatal("expected repository create to be called")
	}
	if ip.ID != IPAddressID("ip-1") {
		t.Fatalf("unexpected ip id: %v", ip.ID)
	}
}

func TestDeleteSubnetReturnsNotFoundWhenRepositoryReportsNoDelete(t *testing.T) {
	svc := NewNetworkService(
		stubSubnetRepository{
			deleteFn: func(context.Context, int64) (bool, error) {
				return false, nil
			},
		},
		stubIPRepository{},
	)

	err := svc.DeleteSubnet(context.Background(), 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
