package db

import (
	"context"
	"fmt"
	"time"

	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type KubernetesDiscoveryRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewKubernetesDiscoveryRepository(pool *pgxpool.Pool) *KubernetesDiscoveryRepository {
	return &KubernetesDiscoveryRepository{pool: pool, queries: sqlc.New(pool)}
}

func (r *KubernetesDiscoveryRepository) Reconcile(ctx context.Context, source domain.KubernetesSourceConfig, services []domain.KubernetesServiceSnapshot, observedAt time.Time) (result domain.KubernetesReconcileResult, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := sqlc.New(tx)
	locked, err := queries.TryKubernetesSourceLock(ctx, source.Key)
	if err != nil {
		return result, err
	}
	if !locked {
		return result, domain.ErrDiscoveryBusy
	}

	sourceRow, err := upsertKubernetesSource(ctx, queries, source)
	if err != nil {
		return result, err
	}

	uids := make([]string, 0, len(services))
	for _, snapshot := range services {
		if snapshot.UID == "" {
			return result, fmt.Errorf("%w: kubernetes service UID is required", domain.ErrInvalidInput)
		}
		serviceRow, upsertErr := queries.UpsertKubernetesService(ctx, sqlc.UpsertKubernetesServiceParams{
			SourceID:        sourceRow.ID,
			KubernetesUid:   snapshot.UID,
			Namespace:       snapshot.Namespace,
			Name:            snapshot.Name,
			ServiceType:     snapshot.Type,
			ResourceVersion: snapshot.ResourceVersion,
			ExternalName:    snapshot.ExternalName,
			DnsName:         snapshot.DNSName,
			ObservedAt:      timestamp(observedAt),
		})
		if upsertErr != nil {
			return result, upsertErr
		}
		uids = append(uids, snapshot.UID)
		if len(snapshot.Addresses) == 0 {
			result.NoUsableIP++
		}

		if err = replaceKubernetesServicePorts(ctx, queries, serviceRow.ID, snapshot.Ports); err != nil {
			return result, err
		}
		if err = replaceKubernetesServiceAddresses(ctx, queries, source.SiteID, serviceRow.ID, snapshot.Addresses, &result); err != nil {
			return result, err
		}
		if err = replaceKubernetesServiceHostnames(ctx, queries, serviceRow.ID, snapshot.Hostnames); err != nil {
			return result, err
		}
	}

	result.Services = len(services)
	if err = queries.MarkMissingKubernetesServicesInactive(ctx, sqlc.MarkMissingKubernetesServicesInactiveParams{
		SourceID: sourceRow.ID,
		Column2:  uids,
		StaleAt:  timestamp(observedAt),
	}); err != nil {
		return result, err
	}
	if err = queries.DeleteStaleKubernetesServices(ctx, sqlc.DeleteStaleKubernetesServicesParams{
		SourceID: sourceRow.ID, StaleAt: timestamp(observedAt.Add(-source.StaleRetention)),
	}); err != nil {
		return result, err
	}
	if err = queries.RecordKubernetesSourceSuccess(ctx, sqlc.RecordKubernetesSourceSuccessParams{
		ID:              sourceRow.ID,
		LastAttemptAt:   timestamp(observedAt),
		ServiceCount:    int32(result.Services),
		MatchedCount:    int32(result.Matched),
		UnmatchedCount:  int32(result.Unmatched),
		AmbiguousCount:  int32(result.Ambiguous),
		NoUsableIpCount: int32(result.NoUsableIP),
	}); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (r *KubernetesDiscoveryRepository) RecordFailure(ctx context.Context, source domain.KubernetesSourceConfig, attemptedAt time.Time, message string) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := sqlc.New(tx)
	locked, err := queries.TryKubernetesSourceLock(ctx, source.Key)
	if err != nil {
		return err
	}
	if !locked {
		return domain.ErrDiscoveryBusy
	}
	if err = ensureKubernetesSource(ctx, queries, source); err != nil {
		return err
	}
	sourceRow, err := queries.GetKubernetesSourceByKey(ctx, source.Key)
	if err != nil {
		return err
	}
	if err = queries.RecordKubernetesSourceFailure(ctx, sqlc.RecordKubernetesSourceFailureParams{
		ID: sourceRow.ID, LastAttemptAt: timestamp(attemptedAt), LastError: message,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *KubernetesDiscoveryRepository) ListSourceStatuses(ctx context.Context) ([]domain.KubernetesSourceStatus, error) {
	rows, err := r.queries.ListKubernetesSourceStatuses(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make([]domain.KubernetesSourceStatus, 0, len(rows))
	for _, row := range rows {
		state := "pending"
		if row.LastError != "" {
			state = "degraded"
		} else if row.LastSuccessAt.Valid {
			state = "healthy"
		}
		statuses = append(statuses, domain.KubernetesSourceStatus{
			Source:        domain.KubernetesSource{Key: row.SourceKey, Name: row.Name},
			SiteID:        uuid.UUID(row.SiteID.Bytes),
			ClusterDomain: row.ClusterDomain,
			Namespaces:    append([]string(nil), row.NamespaceScope...),
			State:         state,
			LastAttemptAt: optionalTime(row.LastAttemptAt),
			LastSuccessAt: optionalTime(row.LastSuccessAt),
			LastError:     row.LastError,
			Services:      int(row.ServiceCount),
			Matched:       int(row.MatchedCount),
			Unmatched:     int(row.UnmatchedCount),
			Ambiguous:     int(row.AmbiguousCount),
			NoUsableIP:    int(row.NoUsableIpCount),
		})
	}
	return statuses, nil
}

func (r *KubernetesDiscoveryRepository) ListServicesBySubnetID(ctx context.Context, subnetID int64) (map[domain.IPAddressID][]domain.KubernetesServiceEnrichment, error) {
	rows, err := r.queries.ListMatchedKubernetesServicesBySubnet(ctx, subnetID)
	if err != nil {
		return nil, err
	}
	result := make(map[domain.IPAddressID][]domain.KubernetesServiceEnrichment)
	indexes := make(map[domain.IPAddressID]map[string]int)
	for _, row := range rows {
		ipID := domain.IPAddressID(uuid.UUID(row.IpAddressID.Bytes).String())
		serviceKey := row.SourceKey + "\x00" + row.KubernetesUid
		if indexes[ipID] == nil {
			indexes[ipID] = make(map[string]int)
		}
		index, ok := indexes[ipID][serviceKey]
		if !ok {
			index = len(result[ipID])
			indexes[ipID][serviceKey] = index
			result[ipID] = append(result[ipID], domain.KubernetesServiceEnrichment{
				Source:           domain.KubernetesSource{Key: row.SourceKey, Name: row.SourceName},
				UID:              row.KubernetesUid,
				Name:             row.Name,
				Namespace:        row.Namespace,
				Type:             row.ServiceType,
				DNSName:          row.DnsName,
				ObservedAt:       row.ObservedAt.Time,
				MatchedAddresses: make([]domain.KubernetesMatchedAddress, 0),
				Ports:            make([]domain.KubernetesServicePort, 0),
			})
		}
		service := &result[ipID][index]
		address := domain.KubernetesMatchedAddress{IP: row.Address, Kind: row.AddressKind}
		if !containsMatchedAddress(service.MatchedAddresses, address) {
			service.MatchedAddresses = append(service.MatchedAddresses, address)
		}
		if row.Port.Valid {
			port := domain.KubernetesServicePort{
				Name: row.PortName.String, Protocol: row.Protocol.String, Port: row.Port.Int32,
				TargetPort: row.TargetPort.String, AppProtocol: row.AppProtocol.String,
			}
			if row.NodePort.Valid {
				nodePort := row.NodePort.Int32
				port.NodePort = &nodePort
			}
			if !containsServicePort(service.Ports, port) {
				service.Ports = append(service.Ports, port)
			}
		}
	}
	return result, nil
}

func (r *KubernetesDiscoveryRepository) ListAllServicesBySubnetID(ctx context.Context, subnetID int64) ([]domain.KubernetesServiceObservation, error) {
	serviceRows, err := r.queries.ListActiveKubernetesServicesBySubnet(ctx, subnetID)
	if err != nil {
		return nil, err
	}
	services := make([]domain.KubernetesServiceObservation, 0, len(serviceRows))
	indexes := make(map[uuid.UUID]int, len(serviceRows))
	for _, row := range serviceRows {
		serviceID := uuid.UUID(row.ServiceID.Bytes)
		indexes[serviceID] = len(services)
		services = append(services, domain.KubernetesServiceObservation{
			Source:       domain.KubernetesSource{Key: row.SourceKey, Name: row.SourceName},
			UID:          row.KubernetesUid,
			Name:         row.Name,
			Namespace:    row.Namespace,
			Type:         row.ServiceType,
			ExternalName: row.ExternalName,
			DNSName:      row.DnsName,
			Addresses:    make([]domain.KubernetesAddressObservation, 0),
			Hostnames:    make([]domain.KubernetesServiceHostname, 0),
			Ports:        make([]domain.KubernetesServicePort, 0),
			ObservedAt:   row.ObservedAt.Time,
		})
	}

	addressRows, err := r.queries.ListKubernetesServiceAddressesBySubnet(ctx, subnetID)
	if err != nil {
		return nil, err
	}
	for _, row := range addressRows {
		index, ok := indexes[uuid.UUID(row.ServiceID.Bytes)]
		if !ok {
			continue
		}
		address := domain.KubernetesAddressObservation{
			IP:          row.Address,
			Kind:        row.Kind,
			IPMode:      row.IpMode,
			MatchStatus: domain.KubernetesMatchStatus(row.MatchStatus),
			MatchCount:  int(row.MatchCount),
		}
		if row.IpAddressID.Valid {
			id := domain.IPAddressID(uuid.UUID(row.IpAddressID.Bytes).String())
			address.MatchedIPAddressID = &id
			if row.MatchedSubnetID.Valid {
				subnetID := row.MatchedSubnetID.Int64
				address.MatchedSubnetID = &subnetID
			}
		}
		services[index].Addresses = append(services[index].Addresses, address)
	}

	portRows, err := r.queries.ListKubernetesServicePortsBySubnet(ctx, subnetID)
	if err != nil {
		return nil, err
	}
	for _, row := range portRows {
		index, ok := indexes[uuid.UUID(row.ServiceID.Bytes)]
		if !ok {
			continue
		}
		port := domain.KubernetesServicePort{
			Name: row.Name, Protocol: row.Protocol, Port: row.Port,
			TargetPort: row.TargetPort, AppProtocol: row.AppProtocol,
		}
		if row.NodePort.Valid {
			nodePort := row.NodePort.Int32
			port.NodePort = &nodePort
		}
		services[index].Ports = append(services[index].Ports, port)
	}

	hostnameRows, err := r.queries.ListKubernetesServiceHostnamesBySubnet(ctx, subnetID)
	if err != nil {
		return nil, err
	}
	for _, row := range hostnameRows {
		index, ok := indexes[uuid.UUID(row.ServiceID.Bytes)]
		if !ok {
			continue
		}
		services[index].Hostnames = append(services[index].Hostnames, domain.KubernetesServiceHostname{
			Kind: row.Kind, Hostname: row.Hostname,
		})
	}

	for i := range services {
		services[i].MatchStatus = kubernetesServiceMatchStatus(services[i].Addresses)
	}
	return services, nil
}

func upsertKubernetesSource(ctx context.Context, queries *sqlc.Queries, source domain.KubernetesSourceConfig) (sqlc.KubernetesSource, error) {
	return queries.UpsertKubernetesSource(ctx, sqlc.UpsertKubernetesSourceParams{
		SourceKey: source.Key, Name: source.Name, SiteID: siteIDParam(&source.SiteID),
		ClusterDomain: source.ClusterDomain, Column5: source.Namespaces,
	})
}

func ensureKubernetesSource(ctx context.Context, queries *sqlc.Queries, source domain.KubernetesSourceConfig) error {
	return queries.EnsureKubernetesSource(ctx, sqlc.EnsureKubernetesSourceParams{
		SourceKey: source.Key, Name: source.Name, SiteID: siteIDParam(&source.SiteID),
		ClusterDomain: source.ClusterDomain, Column5: source.Namespaces,
	})
}

func replaceKubernetesServicePorts(ctx context.Context, queries *sqlc.Queries, serviceID pgtype.UUID, ports []domain.KubernetesServicePort) error {
	if err := queries.DeleteKubernetesServicePorts(ctx, serviceID); err != nil {
		return err
	}
	for _, port := range ports {
		var nodePort int32
		if port.NodePort != nil {
			nodePort = *port.NodePort
		}
		if err := queries.CreateKubernetesServicePort(ctx, sqlc.CreateKubernetesServicePortParams{
			ServiceID: serviceID, Column2: port.Name, Protocol: port.Protocol, Port: port.Port,
			TargetPort: port.TargetPort, Column6: port.AppProtocol, Column7: nodePort,
		}); err != nil {
			return err
		}
	}
	return nil
}

func replaceKubernetesServiceAddresses(ctx context.Context, queries *sqlc.Queries, siteID uuid.UUID, serviceID pgtype.UUID, addresses []domain.KubernetesServiceAddress, result *domain.KubernetesReconcileResult) error {
	if err := queries.DeleteKubernetesServiceAddresses(ctx, serviceID); err != nil {
		return err
	}
	for _, address := range addresses {
		candidates, err := queries.FindIPCandidatesBySiteAndAddress(ctx, sqlc.FindIPCandidatesBySiteAndAddressParams{
			SiteID: siteIDParam(&siteID), Column2: address.Address,
		})
		if err != nil {
			return err
		}
		status := "unmatched"
		var ipAddressID pgtype.UUID
		switch len(candidates) {
		case 0:
			result.Unmatched++
		case 1:
			status = "matched"
			ipAddressID = candidates[0]
			result.Matched++
		default:
			status = "ambiguous"
			result.Ambiguous++
		}
		if err := queries.CreateKubernetesServiceAddress(ctx, sqlc.CreateKubernetesServiceAddressParams{
			ServiceID: serviceID, Kind: address.Kind, Address: address.Address, Column4: address.IPMode,
			IpAddressID: ipAddressID, MatchStatus: status, MatchCount: int32(len(candidates)),
		}); err != nil {
			return err
		}
	}
	return nil
}

func replaceKubernetesServiceHostnames(ctx context.Context, queries *sqlc.Queries, serviceID pgtype.UUID, hostnames []domain.KubernetesServiceHostname) error {
	if err := queries.DeleteKubernetesServiceHostnames(ctx, serviceID); err != nil {
		return err
	}
	for _, hostname := range hostnames {
		if err := queries.CreateKubernetesServiceHostname(ctx, sqlc.CreateKubernetesServiceHostnameParams{
			ServiceID: serviceID, Kind: hostname.Kind, Hostname: hostname.Hostname,
		}); err != nil {
			return err
		}
	}
	return nil
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	timestamp := value.Time.UTC()
	return &timestamp
}

func containsMatchedAddress(addresses []domain.KubernetesMatchedAddress, candidate domain.KubernetesMatchedAddress) bool {
	for _, address := range addresses {
		if address.Kind == candidate.Kind && address.IP == candidate.IP {
			return true
		}
	}
	return false
}

func containsServicePort(ports []domain.KubernetesServicePort, candidate domain.KubernetesServicePort) bool {
	for _, port := range ports {
		if port.Name == candidate.Name && port.Protocol == candidate.Protocol && port.Port == candidate.Port &&
			port.TargetPort == candidate.TargetPort && port.AppProtocol == candidate.AppProtocol && equalOptionalInt32(port.NodePort, candidate.NodePort) {
			return true
		}
	}
	return false
}

func equalOptionalInt32(left, right *int32) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func kubernetesServiceMatchStatus(addresses []domain.KubernetesAddressObservation) domain.KubernetesMatchStatus {
	if len(addresses) == 0 {
		return domain.KubernetesMatchNoUsableIP
	}
	status := domain.KubernetesMatchUnmatched
	for _, address := range addresses {
		if address.MatchStatus == domain.KubernetesMatchMatched {
			return domain.KubernetesMatchMatched
		}
		if address.MatchStatus == domain.KubernetesMatchAmbiguous {
			status = domain.KubernetesMatchAmbiguous
		}
	}
	return status
}

var _ domain.KubernetesDiscoveryRepository = (*KubernetesDiscoveryRepository)(nil)
