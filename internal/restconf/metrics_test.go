package restconf

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/observability"
)

func TestInstrumentRecordsRESTCONF(t *testing.T) {
	reg := observability.NewRegistry()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := Instrument(inner, reg)
	req := httptest.NewRequest(http.MethodGet, "/restconf/data", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	v, ok := reg.Get(observability.MetricRESTCONFRequestsTotal, map[string]string{"method": "GET", "code": "200"})
	if !ok || v != 1 {
		t.Fatalf("got %v %v", v, ok)
	}
	var b strings.Builder
	if err := reg.WriteOpenMetrics(&b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "labnetconf_restconf_requests_total") {
		t.Fatal(b.String())
	}
}
