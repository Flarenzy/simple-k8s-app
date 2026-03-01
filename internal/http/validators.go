package http

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
)

func parsePathInt64(r *http.Request, name string) (int64, error) {
	v := r.PathValue(name)
	if v == "" {
		return 0, fmt.Errorf("%s missing", name)
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s invalid: %w", name, err)
	}
	return id, nil
}

func parseIPAddressID(name string) (domain.IPAddressID, error) {
	_, err := uuid.Parse(name)
	if err != nil {
		return "", err
	}
	return domain.IPAddressID(name), nil
}
