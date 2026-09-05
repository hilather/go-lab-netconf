package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, instance string, dst any) bool {
	if err := s.checkJSONContentType(r); err != nil {
		s.writeProblem(w, r, instance, err)
		return false
	}
	return s.decodeJSONBody(w, r, instance, dst, true)
}

func (s *Server) decodeJSONOptional(w http.ResponseWriter, r *http.Request, instance string, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeProblem(w, r, instance, decodeError(err))
		return false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return true
	}
	if err := s.checkJSONContentType(r); err != nil {
		s.writeProblem(w, r, instance, err)
		return false
	}
	return s.decodeBytes(w, r, instance, body, dst)
}

func (s *Server) decodeJSONBody(w http.ResponseWriter, r *http.Request, instance string, dst any, required bool) bool {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeProblem(w, r, instance, decodeError(err))
		return false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		if required {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("request body is required",
				domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "request body is required"}))
			return false
		}
		return true
	}
	return s.decodeBytes(w, r, instance, body, dst)
}

func (s *Server) decodeBytes(w http.ResponseWriter, r *http.Request, instance string, body []byte, dst any) bool {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		s.writeProblem(w, r, instance, decodeError(err))
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		s.writeProblem(w, r, instance, domainerr.ValidationFailed("request body must contain a single JSON value",
			domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "trailing JSON is not allowed"}))
		return false
	}
	return true
}

func (s *Server) readRawBody(w http.ResponseWriter, r *http.Request, instance string, required bool) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeProblem(w, r, instance, decodeError(err))
		return nil, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		if required {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("request body is required",
				domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "request body is required"}))
			return nil, false
		}
		return nil, true
	}
	return body, true
}

func (s *Server) checkJSONContentType(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return domainerr.ValidationFailed("content type is required",
			domainerr.FieldViolation{Path: "content-type", Code: "required", Message: "POST bodies must be application/json"})
	}
	media := strings.TrimSpace(strings.Split(ct, ";")[0])
	if !strings.EqualFold(media, "application/json") {
		return domainerr.ValidationFailed("unsupported content type",
			domainerr.FieldViolation{Path: "content-type", Code: "invalid_value", Message: "expected application/json"})
	}
	return nil
}

func decodeError(err error) error {
	if err == nil {
		return nil
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return domainerr.ValidationFailed("request body too large",
			domainerr.FieldViolation{Path: "", Code: "document_too_large", Message: "request body exceeds the management limit"})
	}
	msg := "invalid JSON"
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		msg = "request body is required"
	}
	return domainerr.ValidationFailed(msg,
		domainerr.FieldViolation{Path: "", Code: "invalid_value", Message: "request body is not valid JSON"})
}

func expectedRevision(r *http.Request, body string) string {
	if body != "" {
		return body
	}
	if v := strings.Trim(r.Header.Get(headerIfMatch), `"`); v != "" && !strings.EqualFold(v, "*") {
		return v
	}
	return strings.TrimSpace(r.Header.Get(headerExpected))
}

func idempotencyKey(r *http.Request, body string) string {
	if body != "" {
		return body
	}
	return strings.TrimSpace(r.Header.Get(headerIdempotency))
}
