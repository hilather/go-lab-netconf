package nctest

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/hilather/go-lab-netconf/internal/ncserver"
)

func TestNCClientInterop(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	out, err := exec.Command("python3", "-c", "import ncclient").CombinedOutput()
	if err != nil {
		t.Skipf("ncclient not installed: %s", bytesTrim(out))
	}
	addr := startStack(t, "127.0.0.1:0", []ncserver.User{{
		Name:       "alice",
		Profile:    "router-a",
		Handle:     hostnameHandle(t, "router-a", "lab-rtr-a"),
		Namespaces: map[string]string{"ietf-system": "urn:ietf:params:xml:ns:yang:ietf-system"},
	}}, nil)
	host, port, ok := strings.Cut(addr, ":")
	if !ok {
		t.Fatalf("addr %s", addr)
	}
	script := `
from ncclient import manager
with manager.connect(host="` + host + `", port=` + port + `, username="alice", password="alice-lab-password", hostkey_verify=False, allow_agent=False, look_for_keys=False, device_params={"name": "default"}) as m:
    c = m.get_config(source="running")
    xml = c.xml
    if "lab-rtr-a" not in xml:
        raise SystemExit("missing hostname: " + xml)
`
	cmd := exec.Command("python3", "-c", script)
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ncclient: %v\n%s", err, got)
	}
}

func bytesTrim(b []byte) string {
	return strings.TrimSpace(string(b))
}
