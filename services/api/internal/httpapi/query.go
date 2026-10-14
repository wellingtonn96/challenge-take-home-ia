package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/tucows/interview-exercise-ai/services/api/internal/apierror"
	"github.com/tucows/interview-exercise-ai/services/api/internal/ragclient"
)

type queryBody struct {
	Question string `json:"question"`
	TopK     *int   `json:"top_k"`
}

func (h *handler) query(w http.ResponseWriter, r *http.Request) error {
	var body queryBody
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	question := strings.TrimSpace(body.Question)
	if question == "" {
		return apierror.Invalid("question is required")
	}
	if body.TopK != nil && (*body.TopK < 1 || *body.TopK > 50) {
		return apierror.Invalid("top_k must be between 1 and 50")
	}

	ctx := r.Context()
	h.flow(ctx, slog.LevelInfo, "rag.query", "calling RAG query",
		"question_len", len(question),
	)
	resp, err := h.rag.Query(ctx, ragclient.QueryRequest{
		Question: question,
		TopK:     body.TopK,
	})
	if err != nil {
		h.log.Error("rag query", "err", err)
		return mapRAGError(err)
	}
	if resp.Sources == nil {
		resp.Sources = []ragclient.SourceChunk{}
	}
	h.flow(ctx, slog.LevelInfo, "query.ok", "query pipeline finished",
		"sources", len(resp.Sources),
		"answer_len", len(resp.Answer),
	)
	writeJSON(w, http.StatusOK, resp)
	return nil
}
