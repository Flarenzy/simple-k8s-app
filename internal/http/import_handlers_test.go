package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

type importServiceStub struct {
	result domain.ImportResult
	err    error
}

func (s importServiceStub) ImportCSV(context.Context, io.Reader) (domain.ImportResult, error) {
	return s.result, s.err
}

func csvUploadRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	part, err := writer.CreateFormFile("file", "inventory.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/import/csv", &buffer)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestImportCSVRouteReturnsStructuredSummary(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)), stubHealthChecker{}, nil, nil, nil)
	api.ImportService = importServiceStub{result: domain.ImportResult{
		Processed: 3, Created: 2, Updated: 1, Failed: 0,
	}}
	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, csvUploadRequest(t, "site,cidr,ip,description\n"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var result ImportResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Processed != 3 || result.Created != 2 || result.Updated != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestImportCSVRouteRequiresMultipartFile(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)), stubHealthChecker{}, nil, nil, nil)
	api.ImportService = importServiceStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/import/csv", bytes.NewBufferString("site,cidr,ip,description\n"))
	api.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestImportCSVRouteRejectsOversizedRequestBeforeCallingService(t *testing.T) {
	api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)), stubHealthChecker{}, nil, nil, nil)
	api.ImportService = importServiceStub{result: domain.ImportResult{Created: 1}}
	request := csvUploadRequest(t, "site,cidr,ip,description\n")
	request.ContentLength = domain.MaxCSVImportBytes + maxCSVMultipartOverhead + 1
	recorder := httptest.NewRecorder()
	api.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.TrimSpace(recorder.Body.String()) == "" {
		t.Fatal("expected an oversized upload error")
	}
}

func TestImportCSVRouteMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "invalid input", serviceErr: domain.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{name: "internal error", serviceErr: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewAPI(slog.New(slog.NewTextHandler(io.Discard, nil)), stubHealthChecker{}, nil, nil, nil)
			api.ImportService = importServiceStub{err: tt.serviceErr}
			recorder := httptest.NewRecorder()
			api.Router().ServeHTTP(recorder, csvUploadRequest(t, "site,cidr,ip,description\n"))
			if recorder.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, recorder.Code, recorder.Body.String())
			}
		})
	}
}
