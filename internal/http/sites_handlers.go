package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
)

// @Summary List sites
// @Tags sites
// @Security BearerAuth
// @Produce json
// @Success 200 {array} SiteResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sites [get]
func (a *API) handleGetAllSites(w http.ResponseWriter, r *http.Request) {
	statistics, err := a.SitesService.Statistics(r.Context())
	if err != nil {
		a.writeSiteError(w, r, http.StatusInternalServerError, "internal server error", "listing sites", err)
		return
	}
	a.writeJSON(w, r, http.StatusOK, siteStatisticsToSiteResponses(statistics))
}

// @Summary Create site
// @Tags sites
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param site body SiteRequest true "Site payload"
// @Success 201 {object} SiteResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sites [post]
func (a *API) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	request, err := decodeSiteRequest(r)
	if err != nil {
		a.writeSiteError(w, r, http.StatusBadRequest, "bad request", "decoding site", err)
		return
	}
	site, err := a.SitesService.Create(r.Context(), request.createInput())
	if err != nil {
		a.writeSiteError(w, r, http.StatusInternalServerError, "internal server error", "creating site", err)
		return
	}
	a.writeJSON(w, r, http.StatusCreated, siteToResponse(site))
}

// @Summary Get site statistics
// @Tags sites
// @Security BearerAuth
// @Produce json
// @Success 200 {array} SiteStatisticsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sites/statistics [get]
func (a *API) handleGetSiteStatistics(w http.ResponseWriter, r *http.Request) {
	statistics, err := a.SitesService.Statistics(r.Context())
	if err != nil {
		a.writeSiteError(w, r, http.StatusInternalServerError, "internal server error", "reading site statistics", err)
		return
	}
	a.writeJSON(w, r, http.StatusOK, siteStatisticsToResponse(statistics))
}

// @Summary Get site by ID
// @Tags sites
// @Security BearerAuth
// @Produce json
// @Param id path string true "Site ID"
// @Success 200 {object} SiteResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sites/{id} [get]
func (a *API) handleGetSiteByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseSiteID(r)
	if err != nil {
		a.writeSiteError(w, r, http.StatusBadRequest, "bad request", "parsing site id", err)
		return
	}
	site, err := a.SitesService.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			a.writeSiteError(w, r, http.StatusNotFound, "site not found", "finding site", err)
			return
		}
		a.writeSiteError(w, r, http.StatusInternalServerError, "internal server error", "finding site", err)
		return
	}
	a.writeJSON(w, r, http.StatusOK, siteToResponse(site))
}

// @Summary Update site
// @Tags sites
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Site ID"
// @Param site body SiteRequest true "Site payload"
// @Success 200 {object} SiteResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sites/{id} [patch]
func (a *API) handleUpdateSite(w http.ResponseWriter, r *http.Request) {
	id, err := parseSiteID(r)
	if err != nil {
		a.writeSiteError(w, r, http.StatusBadRequest, "bad request", "parsing site id", err)
		return
	}
	request, err := decodeSiteRequest(r)
	if err != nil {
		a.writeSiteError(w, r, http.StatusBadRequest, "bad request", "decoding site", err)
		return
	}
	site, err := a.SitesService.Update(r.Context(), request.updateInput(id))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			a.writeSiteError(w, r, http.StatusNotFound, "site not found", "updating site", err)
			return
		}
		a.writeSiteError(w, r, http.StatusInternalServerError, "internal server error", "updating site", err)
		return
	}
	a.writeJSON(w, r, http.StatusOK, siteToResponse(site))
}

// @Summary Delete site
// @Tags sites
// @Security BearerAuth
// @Param id path string true "Site ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/sites/{id} [delete]
func (a *API) handleDeleteSiteByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseSiteID(r)
	if err != nil {
		a.writeSiteError(w, r, http.StatusBadRequest, "bad request", "parsing site id", err)
		return
	}
	deleted, err := a.SitesService.Delete(r.Context(), id)
	if err != nil {
		a.writeSiteError(w, r, http.StatusInternalServerError, "internal server error", "deleting site", err)
		return
	}
	if !deleted {
		a.writeSiteError(w, r, http.StatusNotFound, "site not found", "deleting site", domain.ErrNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeSiteRequest(r *http.Request) (SiteRequest, error) {
	defer r.Body.Close()
	request, err := decode[SiteRequest](r)
	if err != nil {
		return request, err
	}
	if strings.TrimSpace(request.Name) == "" {
		return request, fmt.Errorf("site name is required")
	}
	request.Name = strings.TrimSpace(request.Name)
	return request, nil
}

func parseSiteID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}

func (a *API) writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	if err := encode(w, r, status, value); err != nil {
		a.Logger.ErrorContext(r.Context(), "responding to client", "err", err)
	}
}

func (a *API) writeSiteError(w http.ResponseWriter, r *http.Request, status int, message, operation string, cause error) {
	a.Logger.ErrorContext(r.Context(), operation, "err", cause)
	a.writeJSON(w, r, status, ErrorResponse{Error: message})
}
