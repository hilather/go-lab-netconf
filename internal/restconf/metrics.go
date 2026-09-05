package restconf

import (
	"net/http"

	"github.com/hilather/go-lab-netconf/internal/observability"
)

// Record increments labnetconf_restconf_requests_total. A nil registry is a no-op.
func Record(reg *observability.Registry, method string, status int) {
	observability.ObserveRESTCONF(reg, method, status)
}

// Instrument wraps next and records RESTCONF request metrics.
func Instrument(next http.Handler, reg *observability.Registry) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)
		Record(reg, r.Method, sw.code)
	})
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(status int) {
	w.code = status
	w.ResponseWriter.WriteHeader(status)
}
