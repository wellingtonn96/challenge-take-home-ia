package httpapi

import (
	"net/http"

	"github.com/tucows/interview-exercise-ai/services/api/internal/apierror"
)

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) error {
	if err := h.repo.Ping(r.Context()); err != nil {
		h.log.Error("database ping", "err", err)
		return apierror.Unavailable("database is unavailable")
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Database: "ok"})
	return nil
}
