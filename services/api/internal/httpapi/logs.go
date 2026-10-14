package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/tucows/interview-exercise-ai/services/api/internal/apierror"
	"github.com/tucows/interview-exercise-ai/services/api/internal/obslog"
)

type logsResponse struct {
	Service string         `json:"service"`
	Entries []obslog.Entry `json:"entries"`
}

func (h *handler) logs(w http.ResponseWriter, r *http.Request) error {
	since := time.Time{}
	if raw := r.URL.Query().Get("since"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return apierror.Invalid("since must be RFC3339Nano timestamp")
		}
		since = parsed.UTC()
	}
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := parsePositiveInt(raw, "limit", 500)
		if err != nil {
			return err
		}
		limit = n
	}
	writeJSON(w, http.StatusOK, logsResponse{
		Service: "api",
		Entries: obslog.DefaultRing().Snapshot(since, limit),
	})
	return nil
}

func parsePositiveInt(raw, name string, max int) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, apierror.Invalid(name + " must be a positive integer")
	}
	if n > max {
		return 0, apierror.Invalid(name + " is too large")
	}
	return n, nil
}
