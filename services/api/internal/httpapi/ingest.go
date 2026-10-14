package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/tucows/interview-exercise-ai/services/api/internal/apierror"
	"github.com/tucows/interview-exercise-ai/services/api/internal/documents"
	"github.com/tucows/interview-exercise-ai/services/api/internal/ragclient"
)

type ingestBody struct {
	Title       string `json:"title"`
	Text        string `json:"text"`
	SourceURI   string `json:"source_uri"`
	ContentType string `json:"content_type"`
}

func (h *handler) ingest(w http.ResponseWriter, r *http.Request) error {
	var body ingestBody
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		return apierror.Invalid("title is required")
	}
	if strings.TrimSpace(body.Text) == "" {
		return apierror.Invalid("text is required")
	}
	contentType := strings.TrimSpace(body.ContentType)
	if contentType == "" {
		contentType = "text/plain"
	}

	ctx := r.Context()
	id := uuid.New()
	h.flow(ctx, slog.LevelInfo, "db.insert", "saving document metadata", "document_id", id)
	err := h.repo.Insert(ctx, documents.Document{
		ID:          id,
		Title:       title,
		SourceURI:   strings.TrimSpace(body.SourceURI),
		ContentType: contentType,
	})
	if err != nil {
		if errors.Is(err, documents.ErrConflict) {
			return apierror.Conflict("document already exists")
		}
		h.log.Error("insert document", "err", err)
		return apierror.Internal("could not store document")
	}

	// Metadata is committed first so source_uri survives the Python upsert,
	// which updates title and updated_at only. An HTTP error from RAG did not
	// commit chunks, so the row is removed. A timeout is left in place because
	// that transaction may still commit and recreate the document.
	h.flow(ctx, slog.LevelInfo, "rag.ingest", "calling RAG ingest", "document_id", id)
	resp, err := h.rag.Ingest(ctx, ragclient.IngestRequest{
		DocumentID: id,
		Text:       body.Text,
		Title:      title,
	})
	if err != nil {
		h.log.Error("rag ingest", "err", err, "document_id", id)
		if ragclient.ShouldRollback(err) {
			if delErr := h.repo.Delete(r.Context(), id); delErr != nil && !errors.Is(delErr, documents.ErrNotFound) {
				h.log.Error("rollback document", "err", delErr, "document_id", id)
			}
		}
		return mapRAGError(err)
	}

	h.flow(ctx, slog.LevelInfo, "ingest.ok", "ingest pipeline finished",
		"document_id", id,
		"chunks_created", resp.ChunksCreated,
	)
	writeJSON(w, http.StatusCreated, resp)
	return nil
}
