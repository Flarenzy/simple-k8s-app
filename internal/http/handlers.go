package http

import (
	"net/http"
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
	ctx := r.Context()
	subnets, err := a.Queries.ListSubnets(ctx)
	if err != nil {
		a.Logger.ErrorContext(ctx, "error reading subnets from db", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte("internal server error"))
		if err != nil {
			a.Logger.ErrorContext(ctx, "error writting error response to client", "err", err.Error())
		}
		return
	}
	err = encode(w, r, http.StatusOK, subnets)
	if err != nil {
		a.Logger.ErrorContext(ctx, "error responding to client with subnet list", "err", err.Error())
	}
}
