package domain

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/netip"
	"strings"

	"github.com/google/uuid"
)

// MaxCSVImportBytes is the maximum number of bytes accepted for one CSV import.
const MaxCSVImportBytes int64 = 64 << 20

const (
	maxCSVImportRows = 100_000
	maxCSVFieldBytes = 4 << 10
	maxCSVRowBytes   = 16 << 10
)

type ImportResult struct {
	Processed int        `json:"processed"`
	Created   int        `json:"created"`
	Updated   int        `json:"updated"`
	Failed    int        `json:"failed"`
	Errors    []RowError `json:"errors"`
}

type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type csvImportService struct {
	sites   SitesService
	network NetworkService
}

type importOutcome int

const (
	importUnchanged importOutcome = iota
	importCreated
	importUpdated
)

func NewCSVImportService(sites SitesService, network NetworkService) ImportService {
	return &csvImportService{sites: sites, network: network}
}

func (s *csvImportService) ImportCSV(ctx context.Context, input io.Reader) (ImportResult, error) {
	limitedInput := &io.LimitedReader{R: input, N: MaxCSVImportBytes + 1}
	reader := csv.NewReader(limitedInput)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = false
	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return ImportResult{}, fmt.Errorf("%w: missing header", ErrInvalidInput)
		}
		return ImportResult{}, fmt.Errorf("%w: invalid csv: %v", ErrInvalidInput, err)
	}
	if err := validateCSVHeader(header); err != nil {
		return ImportResult{}, err
	}

	sites, err := s.sites.List(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	siteByName := make(map[string]Site, len(sites))
	for _, site := range sites {
		siteByName[site.Name] = site
	}
	subnets, err := s.network.ListSubnets(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	subnetByKey := make(map[string]Subnet, len(subnets))
	for _, subnet := range subnets {
		if subnet.SiteID != uuid.Nil && subnet.CIDR.IsValid() {
			subnetByKey[subnetKey(subnet.SiteID, subnet.CIDR)] = subnet
		}
	}
	ipsBySubnet := make(map[int64][]IPAddress)
	var result ImportResult
	for rowNumber := 2; ; rowNumber++ {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if rowNumber > maxCSVImportRows+1 {
			return ImportResult{}, fmt.Errorf("%w: maximum row count exceeded", ErrInvalidInput)
		}
		result.Processed++
		if readErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, RowError{Row: rowNumber, Message: "invalid csv row: " + readErr.Error()})
			continue
		}
		if len(row) != 4 {
			result.Failed++
			result.Errors = append(result.Errors, RowError{Row: rowNumber, Message: "expected exactly 4 columns"})
			continue
		}
		if err := validateCSVRowLimits(row); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, RowError{Row: rowNumber, Message: err.Error()})
			continue
		}
		outcome, processErr := s.importRow(ctx, row, siteByName, subnetByKey, ipsBySubnet)
		if processErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, RowError{Row: rowNumber, Message: processErr.Error()})
			continue
		}
		switch outcome {
		case importCreated:
			result.Created++
		case importUpdated:
			result.Updated++
		}
	}
	if limitedInput.N == 0 {
		return ImportResult{}, fmt.Errorf("%w: csv file exceeds maximum size of %d bytes", ErrInvalidInput, MaxCSVImportBytes)
	}
	return result, nil
}

func validateCSVRowLimits(row []string) error {
	total := 0
	for _, field := range row {
		if len(field) > maxCSVFieldBytes {
			return fmt.Errorf("csv field exceeds maximum size of %d bytes", maxCSVFieldBytes)
		}
		total += len(field)
	}
	if total > maxCSVRowBytes {
		return fmt.Errorf("csv row exceeds maximum size of %d bytes", maxCSVRowBytes)
	}
	return nil
}

func validateCSVHeader(header []string) error {
	if len(header) != 4 || header[0] != "site" || header[1] != "cidr" || header[2] != "ip" || header[3] != "description" {
		return fmt.Errorf("%w: header must be exactly site,cidr,ip,description", ErrInvalidInput)
	}
	return nil
}

func (s *csvImportService) importRow(ctx context.Context, row []string, siteByName map[string]Site, subnetByKey map[string]Subnet, ipsBySubnet map[int64][]IPAddress) (importOutcome, error) {
	siteName := strings.TrimSpace(row[0])
	if siteName == "" {
		return importUnchanged, fmt.Errorf("site is required")
	}
	prefix, err := netip.ParsePrefix(strings.TrimSpace(row[1]))
	if err != nil {
		return importUnchanged, fmt.Errorf("invalid cidr")
	}
	prefix = prefix.Masked()
	ip, err := netip.ParseAddr(strings.TrimSpace(row[2]))
	if err != nil {
		return importUnchanged, fmt.Errorf("invalid ip")
	}
	description := row[3]
	if err := validateIPInSubnet(prefix, ip); err != nil {
		return importUnchanged, err
	}

	site, ok := siteByName[siteName]
	if !ok {
		site, err = s.sites.Create(ctx, CreateSiteInput{Name: siteName})
		if err != nil {
			return importUnchanged, fmt.Errorf("create site: %w", err)
		}
		siteByName[siteName] = site
	}
	key := subnetKey(site.ID, prefix)
	subnet, ok := subnetByKey[key]
	if !ok {
		subnet, err = s.network.CreateSubnet(ctx, CreateSubnetInput{CIDR: prefix.String(), SiteID: &site.ID})
		if err != nil {
			return importUnchanged, fmt.Errorf("create subnet: %w", err)
		}
		subnetByKey[key] = subnet
	}
	ips, loaded := ipsBySubnet[subnet.ID]
	if !loaded {
		ips, err = s.network.ListIPs(ctx, subnet.ID)
		if err != nil {
			return importUnchanged, fmt.Errorf("list ips: %w", err)
		}
	}
	for _, existing := range ips {
		if existing.IP == ip {
			if existing.Hostname == description {
				ipsBySubnet[subnet.ID] = ips
				return importUnchanged, nil
			}
			if _, err = s.network.UpdateIPHostname(ctx, subnet.ID, existing.ID, UpdateIPInput{Hostname: description}); err != nil {
				return importUnchanged, fmt.Errorf("update ip: %w", err)
			}
			for i := range ips {
				if ips[i].ID == existing.ID {
					ips[i].Hostname = description
				}
			}
			ipsBySubnet[subnet.ID] = ips
			return importUpdated, nil
		}
	}
	created, err := s.network.CreateIP(ctx, subnet.ID, CreateIPInput{IP: ip.String(), Hostname: description})
	if err != nil {
		return importUnchanged, fmt.Errorf("create ip: %w", err)
	}
	ips = append(ips, created)
	ipsBySubnet[subnet.ID] = ips
	return importCreated, nil
}

func subnetKey(siteID uuid.UUID, prefix netip.Prefix) string {
	return siteID.String() + "|" + prefix.Masked().String()
}
