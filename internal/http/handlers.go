package http

import (
	"fmt"
	"net/http"
)


func handleHealthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ctx := r.Context()
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Ok"))
		if err != nil {
			fmt.Printf("error: %v\n", err.Error())
		}
	})
}