package observability

import "net/http"

// Handler serves OpenMetrics from reg. A nil registry writes only # EOF.
func Handler(reg *Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", OpenMetricsContentType)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if reg == nil {
			_, _ = w.Write([]byte("# EOF\n"))
			return
		}
		_ = reg.WriteOpenMetrics(w)
	})
}
