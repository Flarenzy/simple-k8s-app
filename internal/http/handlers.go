package http

import (
	"errors"
	"io"
	"net/http"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
)

// @Summary Health check
// @Tags health
// @Success 200 {string} string "ok"
// @Router /healthz [get]
func (a *API) handleHealthz(w http.ResponseWriter, _ *http.Request) {
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
	if err := a.Health.Ping(ctx); err != nil {
		a.Logger.Error("db ping failed", "err", err)
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// @Summary List subnets
// @Tags subnets
// @Security BearerAuth
// @Produce json
// @Success 200 {array} SubnetResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets [get]
func (a *API) handleGetAllSubnets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subnets, err := a.Service.ListSubnets(ctx)
	if err != nil {
		a.Logger.ErrorContext(ctx, "reading subnets", "err", err.Error())
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
// @Security BearerAuth
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
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			a.Logger.ErrorContext(ctx, "closing body", "err", err.Error())
		}
	}(r.Body)
	if err != nil {
		a.Logger.ErrorContext(ctx, "unmarshaling subnet from request", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
		}
		return
	}

	respSubnet, err := a.Service.CreateSubnet(ctx, subnetReq.toInput())
	if err != nil {
		status := http.StatusInternalServerError
		resp := ErrorResponse{Error: "internal server error while saving subnet to db"}
		if errors.Is(err, domain.ErrInvalidInput) {
			status = http.StatusBadRequest
			resp = ErrorResponse{Error: "invalid cidr"}
		}

		a.Logger.ErrorContext(ctx, "creating subnet", "err", err.Error(), "cidr", subnetReq.CIDR)
		err = encode(w, r, status, resp)
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
// @Security BearerAuth
// @Produce json
// @Param id path int true "Subnet ID"
// @Success 200 {object} SubnetResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id} [get]
func (a *API) handleGetSubnetByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parsePathInt64(r, "id")
	if err != nil {
		a.Logger.ErrorContext(ctx, "unable to convert string id to int64", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "responding to client", "err", err.Error())
		}
		return
	}

	subnet, err := a.Service.GetSubnet(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "failed to get subnet by id", "id", id, "err", err.Error())
		status := http.StatusInternalServerError
		resp := ErrorResponse{Error: "internal server error"}
		if errors.Is(err, domain.ErrNotFound) {
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

// @Summary Create ip under subnet
// @Tags subnets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Subnet id in which the ip is created."
// @Param payload body CreateIPRequest true "IP address to create."
// @Success 201 {object} IPResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id}/ips [post]
func (a *API) handleCreateIPBySubnetID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parsePathInt64(r, "id")
	if err != nil {
		a.Logger.ErrorContext(ctx, "unable to convert string id to int64", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	ipReq, err := decode[CreateIPRequest](r)
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			a.Logger.ErrorContext(ctx, "closing body", "err", err.Error())
		}
	}(r.Body)
	if err != nil {
		a.Logger.ErrorContext(ctx, "unmarshaling ip from request", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	respIP, err := a.Service.CreateIP(ctx, id, ipReq.toInput())
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			a.Logger.DebugContext(ctx, "tried to enter duplicate ip", "ip", ipReq.IP, "err", err.Error())
			err := encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request, ip exists"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			a.Logger.InfoContext(ctx, "subnet doesn't exist for given id", "id", id)
			err := encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet not found"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			a.Logger.DebugContext(ctx, "invalid ip request", "ip", ipReq.IP, "err", err.Error())
			err := encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		a.Logger.ErrorContext(ctx, "cant create IP address", "err", err, "ip", ipReq.IP)
		err := encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error while creating ip"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}
	a.Logger.DebugContext(ctx, "ip created", "ip", respIP)

	err = encode(w, r, http.StatusCreated, ipToResponse(respIP))
	if err != nil {
		a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
	}
}

// @Summary Get ips by subnet ID
// @Tags subnets
// @Security BearerAuth
// @Produce json
// @Param id path int true "Subnet ID"
// @Success 200 {array} IPResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id}/ips [get]
func (a *API) handleGetIPsBySubnetID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parsePathInt64(r, "id")
	if err != nil {
		a.Logger.ErrorContext(ctx, "unable to convert string id to int64", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	respIPs, err := a.Service.ListIPs(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		resp := ErrorResponse{Error: "internal server error"}
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
			resp = ErrorResponse{Error: "subnet not found"}
		}
		a.Logger.ErrorContext(ctx, "can't list ips by subnet id", "id", id, "err", err.Error())
		err := encode(w, r, status, resp)
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	err = encode(w, r, http.StatusOK, ipsToResponse(respIPs))
	if err != nil {
		a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
	}
}

// @Summary Update ip under subnet
// @Tags subnets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Subnet id in which the ip is updated."
// @Param uuid path string true "UUID of the ip to be updated."
// @Param payload body UpdateIPRequest true "IP address to update"
// @Success 200 {object} IPResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id}/ips/{uuid} [patch]
func (a *API) handleUpdateIPByUUID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parsePathInt64(r, "id")
	if err != nil {
		a.Logger.ErrorContext(ctx, "unable to convert string id to int64", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	strUUID := r.PathValue("uuid")
	reqID, err := parseIPAddressID(strUUID)
	if err != nil {
		a.Logger.ErrorContext(ctx, "invalid uuid", "uuid", strUUID, "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	reqHostname, err := decode[UpdateIPRequest](r)
	if err != nil {
		a.Logger.ErrorContext(ctx, "can't unmarshal hostname in request", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	respIP, err := a.Service.UpdateIPHostname(ctx, id, reqID, reqHostname.toInput())
	if err != nil {
		status := http.StatusInternalServerError
		resp := ErrorResponse{Error: "internal server error"}
		if errors.Is(err, domain.ErrSubnetNotFound) {
			status = http.StatusNotFound
			resp = ErrorResponse{Error: "subnet not found"}
		} else if errors.Is(err, domain.ErrIPNotFound) || errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
			resp = ErrorResponse{Error: "ip not found"}
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			status = http.StatusBadRequest
			resp = ErrorResponse{Error: "bad request"}
		}
		a.Logger.ErrorContext(ctx, "failed to update IP with given UUID", "uuid", string(reqID), "err", err.Error())
		err = encode(w, r, status, resp)
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	err = encode(w, r, http.StatusOK, ipToResponse(respIP))
	if err != nil {
		a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
	}
}

// @Summary Delete ip under subnet
// @Tags subnets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Subnet id in which the ip is deleted."
// @Param uuid path string true "UUID of the ip to be deleted."
// @Success 204 "No content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id}/ips/{uuid} [delete]
func (a *API) handleDeleteIPByUUIDandSubnetID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parsePathInt64(r, "id")
	if err != nil {
		a.Logger.ErrorContext(ctx, "unable to convert string id to int64", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	strUUID := r.PathValue("uuid")
	reqID, err := parseIPAddressID(strUUID)
	if err != nil {
		a.Logger.ErrorContext(ctx, "invalid uuid", "uuid", strUUID, "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	err = a.Service.DeleteIP(ctx, id, reqID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			a.Logger.DebugContext(ctx, "subnet id or ip uuid not found", "id", id, "uuid", string(reqID), "err", err.Error())
			err = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet or ip not found"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		a.Logger.ErrorContext(ctx, "uncaught error while deleting for ip", "id", id, "uuid", string(reqID), "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)

}

// @Summary Delete subnet
// @Tags subnets
// @Security BearerAuth
// @Param id path int true "Subnet ID of the subnet to delete."
// @Success 204 "No content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subnets/{id} [delete]
func (a *API) handleDeleteSubnetByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := parsePathInt64(r, "id")
	if err != nil {
		a.Logger.ErrorContext(ctx, "unable to convert string id to int64", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	err = a.Service.DeleteSubnet(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		resp := ErrorResponse{Error: "internal server error"}
		if errors.Is(err, domain.ErrNotFound) {
			status = http.StatusNotFound
			resp = ErrorResponse{Error: "subnet not found"}
		}
		a.Logger.ErrorContext(ctx, "failed to delete subnet", "id", id, "err", err.Error())
		err = encode(w, r, status, resp)
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
