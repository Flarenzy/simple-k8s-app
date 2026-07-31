package domain

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type importSitesStub struct {
	sites []Site
}

func (s *importSitesStub) List(context.Context) ([]Site, error) {
	return append([]Site(nil), s.sites...), nil
}
func (s *importSitesStub) FindByID(_ context.Context, id uuid.UUID) (Site, error) {
	for _, site := range s.sites {
		if site.ID == id {
			return site, nil
		}
	}
	return Site{}, ErrNotFound
}
func (s *importSitesStub) Create(_ context.Context, input CreateSiteInput) (Site, error) {
	site := Site{ID: uuid.New(), Name: input.Name, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.sites = append(s.sites, site)
	return site, nil
}
func (s *importSitesStub) Update(context.Context, UpdateSiteInput) (Site, error) {
	return Site{}, errors.New("not used")
}
func (s *importSitesStub) Delete(context.Context, uuid.UUID) (bool, error) {
	return false, errors.New("not used")
}
func (s *importSitesStub) Statistics(context.Context) ([]SiteStatistics, error) {
	return nil, errors.New("not used")
}

type importNetworkStub struct {
	subnets    []Subnet
	ips        map[int64][]IPAddress
	nextID     int64
	createdIPs int
	updatedIPs int
}

func (s *importNetworkStub) ListSubnets(context.Context) ([]Subnet, error) {
	return append([]Subnet(nil), s.subnets...), nil
}
func (s *importNetworkStub) CreateSubnet(_ context.Context, input CreateSubnetInput) (Subnet, error) {
	s.nextID++
	subnet := Subnet{ID: s.nextID, CIDR: mustImportPrefix(input.CIDR), SiteID: *input.SiteID}
	s.subnets = append(s.subnets, subnet)
	return subnet, nil
}
func (s *importNetworkStub) GetSubnet(context.Context, int64) (Subnet, error) {
	return Subnet{}, errors.New("not used")
}
func (s *importNetworkStub) UpdateSubnet(context.Context, UpdateSubnetInput) (Subnet, error) {
	return Subnet{}, errors.New("not used")
}
func (s *importNetworkStub) AssignSubnetSite(context.Context, AssignSubnetSiteInput) (Subnet, error) {
	return Subnet{}, errors.New("not used")
}
func (s *importNetworkStub) DeleteSubnet(context.Context, int64) error { return errors.New("not used") }
func (s *importNetworkStub) ListIPs(_ context.Context, subnetID int64) ([]IPAddress, error) {
	return append([]IPAddress(nil), s.ips[subnetID]...), nil
}
func (s *importNetworkStub) CreateIP(_ context.Context, subnetID int64, input CreateIPInput) (IPAddress, error) {
	ip := IPAddress{ID: IPAddressID(uuid.NewString()), IP: mustImportAddr(input.IP), Hostname: input.Hostname, SubnetID: subnetID}
	s.ips[subnetID] = append(s.ips[subnetID], ip)
	s.createdIPs++
	return ip, nil
}
func (s *importNetworkStub) UpdateIPHostname(_ context.Context, subnetID int64, id IPAddressID, input UpdateIPInput) (IPAddress, error) {
	for i := range s.ips[subnetID] {
		if s.ips[subnetID][i].ID == id {
			s.ips[subnetID][i].Hostname = input.Hostname
			s.updatedIPs++
			return s.ips[subnetID][i], nil
		}
	}
	return IPAddress{}, ErrNotFound
}
func (s *importNetworkStub) DeleteIP(context.Context, int64, IPAddressID) error {
	return errors.New("not used")
}

func mustImportPrefix(value string) netip.Prefix {
	prefix, _ := netip.ParsePrefix(value)
	return prefix
}
func mustImportAddr(value string) netip.Addr { addr, _ := netip.ParseAddr(value); return addr }

func TestCSVImportCreatesHierarchyAndIsIdempotent(t *testing.T) {
	sites := &importSitesStub{}
	network := &importNetworkStub{ips: make(map[int64][]IPAddress)}
	service := NewCSVImportService(sites, network)
	csv := "site,cidr,ip,description\nHQ,10.0.0.0/24,10.0.0.10,printer\nHQ,10.0.0.0/24,10.0.0.11,phone\nHQ,10.0.0.0/24,10.0.0.10,printer-2\nHQ,10.0.0.0/24,10.0.0.10,printer-2\n"
	result, err := service.ImportCSV(context.Background(), strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != 4 || result.Created != 2 || result.Updated != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(sites.sites) != 1 || len(network.subnets) != 1 || len(network.ips[1]) != 2 {
		t.Fatalf("unexpected hierarchy: sites=%d subnets=%d ips=%d", len(sites.sites), len(network.subnets), len(network.ips[1]))
	}
	if network.updatedIPs != 1 {
		t.Fatalf("expected one metadata update, got %d", network.updatedIPs)
	}
}

func TestCSVImportUsesExistingParentsAndReportsRowErrors(t *testing.T) {
	siteID := uuid.New()
	sites := &importSitesStub{sites: []Site{{ID: siteID, Name: "HQ"}}}
	network := &importNetworkStub{subnets: []Subnet{{ID: 9, CIDR: mustImportPrefix("10.0.0.0/24"), SiteID: siteID}}, ips: make(map[int64][]IPAddress), nextID: 9}
	service := NewCSVImportService(sites, network)
	csv := "site,cidr,ip,description\nHQ,10.0.0.0/24,10.0.0.10,ok\nHQ,not-cidr,10.0.0.11,bad\nHQ,10.0.0.0/24,10.0.1.11,outside\n,10.0.0.0/24,10.0.0.12,blank-site\n"
	result, err := service.ImportCSV(context.Background(), strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != 4 || result.Created != 1 || result.Updated != 0 || result.Failed != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Errors) != 3 || result.Errors[0].Row != 3 || result.Errors[1].Row != 4 || result.Errors[2].Row != 5 {
		t.Fatalf("unexpected row errors: %+v", result.Errors)
	}
	if len(sites.sites) != 1 || len(network.subnets) != 1 {
		t.Fatal("invalid rows should not create parents")
	}
}

func TestCSVImportRequiresExactHeader(t *testing.T) {
	service := NewCSVImportService(&importSitesStub{}, &importNetworkStub{ips: make(map[int64][]IPAddress)})
	_, err := service.ImportCSV(context.Background(), strings.NewReader("site,cidr,ip\nHQ,10.0.0.0/24,10.0.0.1\n"))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestCSVImportRejectsOversizedFieldsAsRowErrors(t *testing.T) {
	service := NewCSVImportService(&importSitesStub{}, &importNetworkStub{ips: make(map[int64][]IPAddress)})
	row := "site,cidr,ip,description\nHQ,10.0.0.0/24,10.0.0.1," + strings.Repeat("x", maxCSVFieldBytes+1) + "\n"
	result, err := service.ImportCSV(context.Background(), strings.NewReader(row))
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != 1 || result.Failed != 1 || len(result.Errors) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCSVImportAcceptsMaximumRowCount(t *testing.T) {
	service := NewCSVImportService(&importSitesStub{}, &importNetworkStub{ips: make(map[int64][]IPAddress)})
	var csv strings.Builder
	csv.WriteString("site,cidr,ip,description\n")
	for i := 0; i < maxCSVImportRows; i++ {
		csv.WriteString("HQ,not-cidr,10.0.0.1,bad\n")
	}

	result, err := service.ImportCSV(context.Background(), strings.NewReader(csv.String()))
	if err != nil {
		t.Fatal(err)
	}
	if result.Processed != maxCSVImportRows || result.Failed != maxCSVImportRows {
		t.Fatalf("unexpected result: processed=%d failed=%d", result.Processed, result.Failed)
	}
}

func FuzzCSVImport(f *testing.F) {
	f.Add("site,cidr,ip,description\nHQ,10.0.0.0/24,10.0.0.1,host\n")
	f.Add("site,cidr,ip,description\n\"unterminated\n")
	f.Fuzz(func(t *testing.T, input string) {
		// Keep fuzz cases focused on parser behavior while the production service
		// still enforces the full 64 MiB request bound.
		if len(input) > 1<<20 {
			input = input[:1<<20]
		}
		service := NewCSVImportService(&importSitesStub{}, &importNetworkStub{ips: make(map[int64][]IPAddress)})
		_, _ = service.ImportCSV(context.Background(), strings.NewReader(input))
	})
}
