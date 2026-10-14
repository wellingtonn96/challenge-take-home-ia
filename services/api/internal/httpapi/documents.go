package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/tucows/interview-exercise-ai/services/api/internal/apierror"
	"github.com/tucows/interview-exercise-ai/services/api/internal/documents"
)

type documentListResponse struct {
	Documents []documents.ListItem `json:"documents"`
}

func (h *handler) listDocuments(w http.ResponseWriter, r *http.Request) error {
	items, err := h.repo.List(r.Context())
	if err != nil {
		h.log.Error("list documents", "err", err)
		return apierror.Internal("could not list documents")
	}
	if items == nil {
		items = []documents.ListItem{}
	}
	writeJSON(w, http.StatusOK, documentListResponse{Documents: items})
	return nil
}

func (h *handler) deleteDocument(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return apierror.Invalid("document id must be a UUID")
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, documents.ErrNotFound) {
			return apierror.NotFound("document not found")
		}
		h.log.Error("delete document", "err", err, "document_id", id)
		return apierror.Internal("could not delete document")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
