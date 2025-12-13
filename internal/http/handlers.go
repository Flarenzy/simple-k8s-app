package http

import (
	"net/http"
	sqlc "github.com/Flarenzy/simple-k8s-app/internal/db/sqlc"

)

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

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

func (a *API) handleGetAllSubnets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context();
	subnets, err := a.Queries.ListSubnets(ctx);
	if err != nil {
		a.Logger.ErrorContext(ctx, "error reading subnets from db", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte("internal server error"))
		if err != nil {
			a.Logger.ErrorContext(ctx, "error writting error response to client", "err", err.Error())
		}
		return
	}
	err = encode(w, r, http.StatusOK, subnets);
	if err != nil {
		a.Logger.ErrorContext(ctx, "error responding to client with subnet list", "err", err.Error())
	}
}

func (a *API) handleCreateSubnet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context();
	subnet, err := decode[sqlc.CreateSubnetParams](r);
	defer r.Body.Close()
	if err != nil {
		a.Logger.ErrorContext(ctx, "error unmarshaling subnet from request", "err", err.Error())
		err = encode(w, r, http.StatusBadRequest, "bad request")
		if err != nil {
			a.Logger.ErrorContext(ctx, "error responding to client", "err", err.Error())
		}
		return
	}

	respSubnet, err := a.Queries.CreateSubnet(ctx, subnet)
	if err != nil {
		a.Logger.ErrorContext(ctx, "error inserting subnet into db", "err", err.Error(), "subnet", subnet.Cidr)
		err = encode(w, r, http.StatusInternalServerError, "internal server error while saving subnet to db")
		if err != nil {
			a.Logger.ErrorContext(ctx, "error responding to client", "err", err.Error())
		}
		return
	}
	err = encode(w, r, http.StatusCreated, respSubnet)
	if err != nil {
		a.Logger.ErrorContext(ctx, "error responding to client", "err", err.Error())
	}
}