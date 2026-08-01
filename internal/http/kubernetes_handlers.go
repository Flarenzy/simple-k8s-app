package http

import (
	"net/http"
)

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
