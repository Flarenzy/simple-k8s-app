package http

import (
	"errors"
	"net/http"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

// @Summary List discovered Kubernetes Services for a subnet site
// @Tags kubernetes
// @Security BearerAuth
// @Produce json
// @Param id path int true "Subnet ID"
// @Success 200 {array} KubernetesServiceObservationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id}/kubernetes-services [get]
func (a *API) handleGetKubernetesServicesBySubnetID(w http.ResponseWriter, r *http.Request) {
	ctx, id, _, done := parseID(w, r, a)
	if done {
		return
	}
	if _, err := a.NetService.GetSubnet(ctx, id); err != nil {
		status := http.StatusInternalServerError
		response := ErrorResponse{Error: "internal server error"}
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
			response = ErrorResponse{Error: "subnet not found"}
		}
		a.Logger.ErrorContext(ctx, "reading subnet for kubernetes services", "id", id, "err", err)
		_ = encode(w, r, status, response)
		return
	}
	if a.DiscoveryService == nil {
		_ = encode(w, r, http.StatusOK, make([]KubernetesServiceObservationResponse, 0))
		return
	}
	services, err := a.DiscoveryService.ListServicesBySubnetID(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "listing kubernetes services by subnet", "id", id, "err", err)
		_ = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}
	_ = encode(w, r, http.StatusOK, kubernetesServiceObservationsToResponse(services))
}

// @Summary List Kubernetes discovery source status
// @Tags kubernetes
// @Security BearerAuth
// @Produce json
// @Success 200 {array} KubernetesDiscoveryStatusResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/kubernetes/sources [get]
func (a *API) handleGetKubernetesSources(w http.ResponseWriter, r *http.Request) {
	if a.DiscoveryService == nil {
		_ = encode(w, r, http.StatusOK, make([]KubernetesDiscoveryStatusResponse, 0))
		return
	}
	statuses, err := a.DiscoveryService.ListSourceStatuses(r.Context())
	if err != nil {
		a.Logger.ErrorContext(r.Context(), "listing kubernetes discovery sources", "err", err)
		_ = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}
	_ = encode(w, r, http.StatusOK, kubernetesStatusesToResponse(statuses))
}
