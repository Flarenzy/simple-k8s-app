package domain

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
)

type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	clone := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clone.AddAttrs(attr)
		return true
	})
	h.records = append(h.records, clone)
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

type stubNetworkService struct {
	listSubnetsFn      func(context.Context) ([]Subnet, error)
	createSubnetFn     func(context.Context, CreateSubnetInput) (Subnet, error)
	getSubnetFn        func(context.Context, int64) (Subnet, error)
	deleteSubnetFn     func(context.Context, int64) error
	listIPsFn          func(context.Context, int64) ([]IPAddress, error)
	createIPFn         func(context.Context, int64, CreateIPInput) (IPAddress, error)
	updateIPHostnameFn func(context.Context, int64, IPAddressID, UpdateIPInput) (IPAddress, error)
	deleteIPFn         func(context.Context, int64, IPAddressID) error
}

func (s stubNetworkService) ListSubnets(ctx context.Context) ([]Subnet, error) {
	if s.listSubnetsFn == nil {
		return nil, nil
	}
	return s.listSubnetsFn(ctx)
}

func (s stubNetworkService) CreateSubnet(ctx context.Context, input CreateSubnetInput) (Subnet, error) {
	if s.createSubnetFn == nil {
		return Subnet{}, nil
	}
	return s.createSubnetFn(ctx, input)
}

func (s stubNetworkService) GetSubnet(ctx context.Context, id int64) (Subnet, error) {
	if s.getSubnetFn == nil {
		return Subnet{}, nil
	}
	return s.getSubnetFn(ctx, id)
}

func (s stubNetworkService) DeleteSubnet(ctx context.Context, id int64) error {
	if s.deleteSubnetFn == nil {
		return nil
	}
	return s.deleteSubnetFn(ctx, id)
}

func (s stubNetworkService) ListIPs(ctx context.Context, subnetID int64) ([]IPAddress, error) {
	if s.listIPsFn == nil {
		return nil, nil
	}
	return s.listIPsFn(ctx, subnetID)
}

func (s stubNetworkService) CreateIP(ctx context.Context, subnetID int64, input CreateIPInput) (IPAddress, error) {
	if s.createIPFn == nil {
		return IPAddress{}, nil
	}
	return s.createIPFn(ctx, subnetID, input)
}

func (s stubNetworkService) UpdateIPHostname(ctx context.Context, subnetID int64, id IPAddressID, input UpdateIPInput) (IPAddress, error) {
	if s.updateIPHostnameFn == nil {
		return IPAddress{}, nil
	}
	return s.updateIPHostnameFn(ctx, subnetID, id, input)
}

func (s stubNetworkService) DeleteIP(ctx context.Context, subnetID int64, id IPAddressID) error {
	if s.deleteIPFn == nil {
		return nil
	}
	return s.deleteIPFn(ctx, subnetID, id)
}

func TestLoggingNetworkServiceLogsSubnetCreation(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	service := NewLoggingNetworkService(logger, stubNetworkService{
		createSubnetFn: func(_ context.Context, _ CreateSubnetInput) (Subnet, error) {
			return Subnet{ID: 7}, nil
		},
	})

	_, err := service.CreateSubnet(context.Background(), CreateSubnetInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(handler.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(handler.records))
	}
	if handler.records[0].Level != slog.LevelInfo || handler.records[0].Message != "subnet created" {
		t.Fatalf("unexpected log record: level=%v message=%q", handler.records[0].Level, handler.records[0].Message)
	}
}

func TestLoggingNetworkServiceLogsErrors(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	service := NewLoggingNetworkService(logger, stubNetworkService{
		createIPFn: func(_ context.Context, _ int64, _ CreateIPInput) (IPAddress, error) {
			return IPAddress{}, ErrConflict
		},
	})

	_, err := service.CreateIP(context.Background(), 1, CreateIPInput{IP: "10.0.0.10"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	if len(handler.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(handler.records))
	}
	if handler.records[0].Level != slog.LevelError || handler.records[0].Message != "create ip failed" {
		t.Fatalf("unexpected log record: level=%v message=%q", handler.records[0].Level, handler.records[0].Message)
	}
}

func TestNewLoggingNetworkServiceReturnsNextWhenLoggerNil(t *testing.T) {
	called := false
	next := stubNetworkService{
		createSubnetFn: func(_ context.Context, _ CreateSubnetInput) (Subnet, error) {
			called = true
			return Subnet{ID: 99}, nil
		},
	}
	wrapped := NewLoggingNetworkService(nil, next)
	subnet, err := wrapped.CreateSubnet(context.Background(), CreateSubnetInput{CIDR: "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected wrapped service to delegate to next")
	}
	if subnet.ID != 99 {
		t.Fatalf("unexpected subnet id: %d", subnet.ID)
	}
}

func TestCaptureHandlerStoresIndependentRecords(t *testing.T) {
	handler := &captureHandler{}
	logger := slog.New(handler)
	logger.Info("first")
	logger.Info("second")

	if len(handler.records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(handler.records))
	}
	if !slices.Equal([]string{handler.records[0].Message, handler.records[1].Message}, []string{"first", "second"}) {
		t.Fatalf("unexpected messages: %q, %q", handler.records[0].Message, handler.records[1].Message)
	}
}
