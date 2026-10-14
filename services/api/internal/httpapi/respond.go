package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/tucows/interview-exercise-ai/services/api/internal/apierror"
	"github.com/tucows/interview-exercise-ai/services/api/internal/ragclient"
)

const maxBodyBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeAPIError(w http.ResponseWriter, err *apierror.Error) {
	if err == nil {
		err = apierror.Internal("internal error")
	}
	writeJSON(w, err.Status, err)
}

func requireJSON(r *http.Request) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return apierror.Invalid("Content-Type must be application/json")
	}
	return nil
}

func decodeJSON(r *http.Request, dest any) error {
	if err := requireJSON(r); err != nil {
		return err
	}
	// nil writer: this package writes its own JSON status instead of the
	// default 413 that MaxBytesReader emits on *http.response.
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return invalidJSON(err)
	}
	if dec.More() {
		return apierror.Invalid("request body must contain a single JSON value")
	}
	return nil
}

func invalidJSON(err error) error {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return apierror.Invalid("request body exceeds 1MiB")
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return apierror.Invalid("request body must be valid JSON")
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return apierror.Invalid("request body must be valid JSON")
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return apierror.Invalid("request body has an invalid field type")
	}
	if strings.Contains(err.Error(), "unknown field") {
		return apierror.Invalid("request body contains an unknown field")
	}
	return apierror.Invalid("request body must be valid JSON")
}

// mapRAGError turns a Python failure into the public error envelope.
// Transport failures and RAG 5xx become 502. RAG 4xx keep their status
// (validation, unknown document, empty corpus) with the same machine code.
func mapRAGError(err error) *apierror.Error {
	var upstream *ragclient.UpstreamError
	if errors.As(err, &upstream) {
		code := upstream.Code
		if code == "" {
			code = "rag_error"
		}
		message := upstream.Message
		if message == "" {
			message = "knowledge service request failed"
		}
		if upstream.StatusCode >= 400 && upstream.StatusCode < 500 {
			return &apierror.Error{Status: upstream.StatusCode, Code: code, Message: message}
		}
		return apierror.BadGateway(code, message)
	}
	return apierror.BadGateway(apierror.CodeRAGUnavailable, "knowledge service is unavailable")
}
