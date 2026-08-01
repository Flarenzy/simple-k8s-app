package http

import (
	"errors"
	"net/http"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

// @Summary Get subnet usage reporting settings
// @Tags reporting
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ReportingSettingsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/reporting/settings [get]
func (a *API) handleGetReportingSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.ReportingService.GetSettings(r.Context())
	if err != nil {
		a.Logger.ErrorContext(r.Context(), "reading reporting settings", "err", err)
		_ = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}
	_ = encode(w, r, http.StatusOK, reportingSettingsToResponse(settings))
}

// @Summary Update subnet usage reporting settings
// @Tags reporting
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param settings body ReportingSettingsRequest true "Reporting settings"
// @Success 200 {object} ReportingSettingsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/reporting/settings [patch]
func (a *API) handleUpdateReportingSettings(w http.ResponseWriter, r *http.Request) {
	request, err := decode[ReportingSettingsRequest](r)
	defer r.Body.Close()
	if err != nil {
		_ = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		return
	}
	settings, err := a.ReportingService.UpdateSettings(r.Context(), domain.UpdateReportingSettingsInput{
		Cadence: domain.ReportingCadence(request.Cadence), RetentionDays: request.RetentionDays,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			_ = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		a.Logger.ErrorContext(r.Context(), "updating reporting settings", "err", err)
		_ = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}
	_ = encode(w, r, http.StatusOK, reportingSettingsToResponse(settings))
}

// @Summary Get periodic subnet usage snapshots
// @Tags reporting
// @Security BearerAuth
// @Produce json
// @Param id path int true "Subnet ID"
// @Param range query string false "Bounded history range" default(7d) Enums(24h,7d,30d,90d,180d)
// @Success 200 {object} SubnetUsageHistoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id}/usage-history [get]
func (a *API) handleGetSubnetUsageHistory(w http.ResponseWriter, r *http.Request) {
	ctx, id, _, done := parseID(w, r, a)
	if done {
		return
	}
	usageRange := r.URL.Query().Get("range")
	if usageRange == "" {
		usageRange = "7d"
	}
	history, err := a.ReportingService.GetSubnetUsageHistory(ctx, id, usageRange)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrIPv6Unsupported):
			_ = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrNotFound):
			_ = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet not found"})
		default:
			a.Logger.ErrorContext(ctx, "reading subnet usage history", "subnet_id", id, "err", err)
			_ = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}
	_ = encode(w, r, http.StatusOK, subnetUsageHistoryToResponse(history))
}
