package domain

import (
	"context"
	"log/slog"
)

type loggingNetworkService struct {
	logger *slog.Logger
	next   NetworkService
}

func NewLoggingNetworkService(logger *slog.Logger, next NetworkService) NetworkService {
	if logger == nil || next == nil {
		return next
	}

	return &loggingNetworkService{
		logger: logger,
		next:   next,
	}
}

func (s *loggingNetworkService) ListSubnets(ctx context.Context) ([]Subnet, error) {
	subnets, err := s.next.ListSubnets(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "list subnets failed", "err", err.Error())
	}
	return subnets, err
}

func (s *loggingNetworkService) CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error) {
	subnet, err := s.next.CreateSubnet(ctx, input)
	if err != nil {
		s.logger.ErrorContext(ctx, "create subnet failed", "cidr", input.CIDR, "err", err.Error())
		return Subnet{}, err
	}

	s.logger.InfoContext(ctx, "subnet created", "id", subnet.ID, "cidr", subnet.CIDR.String())
	return subnet, nil
}

func (s *loggingNetworkService) GetSubnet(ctx context.Context, id int64) (Subnet, error) {
	subnet, err := s.next.GetSubnet(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "get subnet failed", "id", id, "err", err.Error())
	}
	return subnet, err
}

func (s *loggingNetworkService) DeleteSubnet(ctx context.Context, id int64) error {
	err := s.next.DeleteSubnet(ctx, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "delete subnet failed", "id", id, "err", err.Error())
		return err
	}

	s.logger.InfoContext(ctx, "subnet deleted", "id", id)
	return nil
}

func (s *loggingNetworkService) ListIPs(ctx context.Context, subnetID int64) ([]IPAddress, error) {
	ips, err := s.next.ListIPs(ctx, subnetID)
	if err != nil {
		s.logger.ErrorContext(ctx, "list ips failed", "subnet_id", subnetID, "err", err.Error())
	}
	return ips, err
}

func (s *loggingNetworkService) CreateIP(ctx context.Context, subnetID int64, input CreateIPInput) (IPAddress, error) {
	ip, err := s.next.CreateIP(ctx, subnetID, input)
	if err != nil {
		s.logger.ErrorContext(ctx, "create ip failed", "subnet_id", subnetID, "ip", input.IP, "err", err.Error())
		return IPAddress{}, err
	}

	s.logger.DebugContext(ctx, "ip created", "subnet_id", subnetID, "ip", ip.IP.String(), "id", string(ip.ID))
	return ip, nil
}

func (s *loggingNetworkService) UpdateIPHostname(ctx context.Context, subnetID int64, id IPAddressID, input UpdateIPInput) (IPAddress, error) {
	ip, err := s.next.UpdateIPHostname(ctx, subnetID, id, input)
	if err != nil {
		s.logger.ErrorContext(ctx, "update ip hostname failed", "subnet_id", subnetID, "ip_id", string(id), "err", err.Error())
	}
	return ip, err
}

func (s *loggingNetworkService) DeleteIP(ctx context.Context, subnetID int64, id IPAddressID) error {
	err := s.next.DeleteIP(ctx, subnetID, id)
	if err != nil {
		s.logger.ErrorContext(ctx, "delete ip failed", "subnet_id", subnetID, "ip_id", string(id), "err", err.Error())
		return err
	}

	s.logger.DebugContext(ctx, "ip deleted", "subnet_id", subnetID, "ip_id", string(id))
	return nil
}
