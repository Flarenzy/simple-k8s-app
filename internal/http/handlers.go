package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
)

// @Summary Health check
// @Tags health
// @Success 200 {string} string "ok"
// @Router /healthz [get]
func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// @Summary Readiness check
// @Tags health
// @Success 200 {string} string "ready"
// @Failure 503 {string} string "db unavailable"
// @Router /readyz [get]
func (a *API) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := a.DB.Ping(ctx); err != nil {
		a.Logger.Error("db ping failed", "err", err)
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// @Summary List subnets
// @Tags subnets
// @Produce json
// @Success 200 {array} SubnetResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets [get]
func (a *API) handleGetAllSubnets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subnets, err := a.Queries.ListSubnets(ctx)
	if err != nil {
		a.Logger.ErrorContext(ctx, "reading subnets from db", "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "couldn't encode response", "err", err)
		}
		return
	}
	err = encode(w, r, http.StatusOK, subnetsToResponse(subnets))
	if err != nil {
		a.Logger.ErrorContext(ctx, "responding to client with subnet list", "err", err.Error())
	}
}

// @Summary Create subnet
// @Tags subnets
// @Accept json
// @Produce json
// @Param subnet body CreateSubnetRequest true "Subnet payload"
// @Success 201 {object} SubnetResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets [post]
func (a *API) handleCreateSubnet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subnetReq, err := decode[CreateSubnetRequest](r)
	defer r.Body.Close()
	if err != nil {
		a.Logger.ErrorContext(ctx, "unmarshaling subnet from request", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
		}
		return
	}

	subnet, err := subnetReq.toParams()
	if err != nil {
		a.Logger.ErrorContext(ctx, "parsing cidr from request", "err", err.Error(), "cidr", subnetReq.CIDR)
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "invalid cidr"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
		}
		return
	}

	respSubnet, err := a.Queries.CreateSubnet(ctx, subnet)
	if err != nil {
		a.Logger.ErrorContext(ctx, "inserting subnet into db", "err", err.Error(), "subnet", subnet.Cidr)
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error while saving subnet to db"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
		}
		return
	}
	err = encode(w, r, http.StatusCreated, subnetToResponse(respSubnet))
	if err != nil {
		a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
	}
}

// @Summary Get subnet by ID
// @Tags subnets
// @Produce json
// @Param id path int true "Subnet ID"
// @Success 200 {object} SubnetResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id} [get]
func (a *API) handleGetSubnetByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	strID := r.PathValue("id")
	id, err := strconv.ParseInt(strID, 10, 64)
	if err != nil {
		a.Logger.ErrorContext(ctx, "converting string id to int64", "strID", strID, "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
		}
		return
	}

	subnet, err := a.Queries.GetSubnetByID(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "subnet with following id not found", "id", id, "err", err.Error())
		status := http.StatusInternalServerError
		resp := ErrorResponse{Error: "internal server error"}
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
			resp = ErrorResponse{Error: "subnet not found"}
		}
		err = encode(w, r, status, resp)
		if err != nil {
			a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
		}
		return
	}

	err = encode(w, r, http.StatusOK, subnetToResponse(subnet))
	if err != nil {
		a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
	}
}
