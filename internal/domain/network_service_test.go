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
	createFn func(context.Context, CreateSubnetRecord) (Subnet, error)
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

func (s stubSubnetRepository) Create(ctx context.Context, input CreateSubnetRecord) (Subnet, error) {
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
	createFn func(context.Context, CreateIPRecord, int64) (IPAddress, error)
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

func (s stubIPRepository) Create(ctx context.Context, input CreateIPRecord, subnetID int64) (IPAddress, error) {
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
			createFn: func(_ context.Context, input CreateIPRecord, subnetID int64) (IPAddress, error) {
				created = true
				return IPAddress{ID: IPAddressID("ip-1"), IP: input.IP, SubnetID: subnetID}, nil
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

func TestCreateIPReturnsSubnetSpecificNotFound(t *testing.T) {
	svc := NewNetworkService(
		stubSubnetRepository{
			findFn: func(context.Context, int64) (Subnet, error) {
				return Subnet{}, ErrNotFound
			},
		},
		stubIPRepository{},
	)

	_, err := svc.CreateIP(context.Background(), 1, CreateIPInput{IP: "10.0.0.10"})
	if !errors.Is(err, ErrSubnetNotFound) {
		t.Fatalf("expected ErrSubnetNotFound, got %v", err)
	}
}

func TestUpdateIPHostnameReturnsIPSpecificNotFound(t *testing.T) {
	svc := NewNetworkService(
		stubSubnetRepository{
			findFn: func(context.Context, int64) (Subnet, error) {
				return Subnet{ID: 1}, nil
			},
		},
		stubIPRepository{
			findFn: func(context.Context, IPAddressID, int64) (IPAddress, error) {
				return IPAddress{}, ErrNotFound
			},
		},
	)

	_, err := svc.UpdateIPHostname(context.Background(), 1, IPAddressID("ip-1"), UpdateIPInput{Hostname: "host"})
	if !errors.Is(err, ErrIPNotFound) {
		t.Fatalf("expected ErrIPNotFound, got %v", err)
	}
}

func TestCreateIPRejectsIPv4BroadcastAddress(t *testing.T) {
	svc := NewNetworkService(
		stubSubnetRepository{
			findFn: func(context.Context, int64) (Subnet, error) {
				prefix, _ := netip.ParsePrefix("10.0.0.0/24")
				return Subnet{ID: 1, CIDR: prefix}, nil
			},
		},
		stubIPRepository{},
	)

	_, err := svc.CreateIP(context.Background(), 1, CreateIPInput{IP: "10.0.0.255"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateIPAllowsSlash31EndpointAddress(t *testing.T) {
	created := false
	svc := NewNetworkService(
		stubSubnetRepository{
			findFn: func(context.Context, int64) (Subnet, error) {
				prefix, _ := netip.ParsePrefix("10.0.0.0/31")
				return Subnet{ID: 1, CIDR: prefix}, nil
			},
		},
		stubIPRepository{
			createFn: func(_ context.Context, input CreateIPRecord, subnetID int64) (IPAddress, error) {
				created = true
				return IPAddress{ID: IPAddressID("ip-2"), IP: input.IP, SubnetID: subnetID}, nil
			},
		},
	)

	_, err := svc.CreateIP(context.Background(), 1, CreateIPInput{IP: "10.0.0.1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !created {
		t.Fatal("expected repository create to be called")
	}
}
