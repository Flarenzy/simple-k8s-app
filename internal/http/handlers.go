package http

import (
	"errors"
	"net/http"
	"strconv"

	db "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"
	"github.com/jackc/pgconn"
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
		a.Logger.ErrorContext(ctx, "unable to convert string id to int64", "strID", strID, "err", err.Error())
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

// @Summary Create ip under subnet
// @Tags subnets
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

	subnetExists, subnet, err := a.subnetExists(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "uncaught error while checking for subnet", "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	if !subnetExists {
		a.Logger.InfoContext(ctx, "subnet doesn't exist for given id", "id", id)
		err = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet not found"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	ipReq, err := decode[CreateIPRequest](r)
	defer r.Body.Close()
	if err != nil {
		a.Logger.ErrorContext(ctx, "unmarshaling ip from request", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	ip, err := ipReq.toParams(id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "can't parse ip from request", "ip", ipReq.IP, "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	err = validateIPInSubnet(subnet, ip.Ip)
	if err != nil {
		a.Logger.DebugContext(ctx, "invalid ip", "ip", ip.Ip.String(), "subnet", subnet.String())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	respIP, err := a.Queries.CreateIPAddress(ctx, ip)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "unique_ip" {
			a.Logger.DebugContext(ctx, "tried to enter duplicate ip", "ip", ip.Ip, "err", err.Error())
			err := encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request, ip exists"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		a.Logger.ErrorContext(ctx, "cant create IP address", "err", err, "ip", ip)
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

	subnetExists, _, err := a.subnetExists(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "uncaught error while checking for subnet", "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	if !subnetExists {
		a.Logger.InfoContext(ctx, "subnet doesn't exist for given id", "id", id)
		err = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet not found"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	respIPs, err := a.Queries.ListIPsBySubnetID(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "can't list ips by subnet id", "id", id, "err", err.Error())
		err := encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	err = encode(w, r, http.StatusOK, ipsToreponse(respIPs))
	if err != nil {
		a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
	}
}

// @Summary Update ip under subnet
// @Tags subnets
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
	reqUuid, err := strToUUID(strUUID)
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

	subnetExists, _, err := a.subnetExists(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "uncaught error while checking for subnet", "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	if !subnetExists {
		a.Logger.InfoContext(ctx, "subnet doesn't exist for given id", "id", id)
		err = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet not found"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	_, err = a.Queries.GetIPByUUIDandSubnetID(ctx, db.GetIPByUUIDandSubnetIDParams{
		ID:       reqUuid,
		SubnetID: id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.Logger.DebugContext(ctx, "ip for uuid not found", "uuid", reqUuid.String(), "err", err.Error())
			err = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "ip not found"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		a.Logger.ErrorContext(ctx, "uncaught error", "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	respIP, err := a.Queries.UpdateIPByUUID(ctx, reqHostname.toParams(reqUuid))
	if err != nil {
		a.Logger.ErrorContext(ctx, "failed to update IP with given UUID", "uuid", reqUuid.String(), "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
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
	reqUuid, err := strToUUID(strUUID)
	if err != nil {
		a.Logger.ErrorContext(ctx, "invalid uuid", "uuid", strUUID, "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, ErrorResponse{Error: "bad request"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	_, err = a.Queries.DeleteIPByUUIDandSubnetID(ctx, db.DeleteIPByUUIDandSubnetIDParams{
		ID:       reqUuid,
		SubnetID: id,
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.Logger.DebugContext(ctx, "subnet id or ip uuid not found", "id", id, "uuid", reqUuid.String(), "err", err.Error())
			err = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet or ip not found"})
			if err != nil {
				a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
			}
			return
		}
		a.Logger.ErrorContext(ctx, "uncaught error while deleting for ip", "id", id, "uuid", reqUuid.String(), "err", err.Error())
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

	numOfDelRows, err := a.Queries.DeleteSubnetByID(ctx, id)
	if err != nil {
		a.Logger.ErrorContext(ctx, "uncaught error while deleting subnet", "id", id, "err", err.Error())
		err = encode(w, r, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	if numOfDelRows == 0 {
		a.Logger.DebugContext(ctx, "subnet id not found", "id", id)
		err = encode(w, r, http.StatusNotFound, ErrorResponse{Error: "subnet not found"})
		if err != nil {
			a.Logger.ErrorContext(ctx, "cant respond to client", "err", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
