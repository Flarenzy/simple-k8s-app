package http

// import (
// 	"log/slog"
// 	"net/http"
// )

// func NewServer(
// 	logger *slog.Logger,
// 	cfg interface{},
// 	dbQueries interface{},
// 	) http.Handler {

// 	mux := http.NewServeMux();
// 	mux = addRoutes(mux);
// 	return mux;
// }

// func addRoutes(mux *http.ServeMux) *http.ServeMux {
// 	mux.Handle("/healthz", handleHealthz());
// 	return mux;
// }
