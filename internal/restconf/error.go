package restconf

import (
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

func writeError(w http.ResponseWriter, err error) {
	if me, ok := err.(*mediaError); ok {
		w.Header().Set("Content-Type", mediaProblem)
		w.WriteHeader(me.status)
		_ = json.NewEncoder(w).Encode(me.err)
		return
	}
	de, ok := domainerr.As(err)
	if !ok {
		de = domainerr.ValidationFailed(err.Error())
	}
	status := statusFor(de.Code)
	w.Header().Set("Content-Type", mediaProblem)
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Basic realm="restconf"`)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(de)
}

func statusFor(code domainerr.Code) int {
	switch code {
	case domainerr.CodeUnauthorized:
		return http.StatusUnauthorized
	case domainerr.CodeForbidden, domainerr.CodeOriginNotAllowed:
		return http.StatusForbidden
	case domainerr.CodeNotFound:
		return http.StatusNotFound
	case domainerr.CodeCandidateDirty, domainerr.CodeLockDenied, domainerr.CodeRevisionMismatch:
		return http.StatusConflict
	case domainerr.CodeNotWritable:
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}

func writeYangJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		writeError(w, domainerr.ValidationFailed("failed to encode JSON"))
		return
	}
	w.Header().Set("Content-Type", MediaYangJSON)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

type mediaError struct {
	status int
	err    *domainerr.Error
}

func (e *mediaError) Error() string { return e.err.Error() }
func (e *mediaError) Unwrap() error { return e.err }

func requireJSONAccept(r *http.Request) error {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return nil
	}
	for _, part := range strings.Split(accept, ",") {
		mt, _, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		switch mt {
		case MediaYangJSON, "*/*", "application/*":
			return nil
		}
	}
	return &mediaError{
		status: http.StatusNotAcceptable,
		err:    domainerr.New(domainerr.CodeValidationFailed, "Accept must include "+MediaYangJSON),
	}
}

func requireYangJSONContent(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return &mediaError{
			status: http.StatusUnsupportedMediaType,
			err:    domainerr.New(domainerr.CodeValidationFailed, "Content-Type must be "+MediaYangJSON),
		}
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil || mt != MediaYangJSON {
		return &mediaError{
			status: http.StatusUnsupportedMediaType,
			err:    domainerr.New(domainerr.CodeValidationFailed, "Content-Type must be "+MediaYangJSON),
		}
	}
	return nil
}
