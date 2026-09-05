package restconf

import (
	"bytes"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/hilather/go-lab-netconf/internal/domainerr"
)

var loopbackCIDRs = []string{"127.0.0.0/8", "::1/128"}

func parseAdmission(cidrs []string) ([]*net.IPNet, bool, error) {
	if cidrs == nil {
		cidrs = loopbackCIDRs
	}
	if len(cidrs) == 0 {
		return nil, true, nil
	}
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, false, domainerr.ValidationFailed("invalid admission CIDR")
		}
		out = append(out, n)
	}
	return out, false, nil
}

func (s *Server) admit(remote string) bool {
	if s.denyAll {
		return false
	}
	ip := ipFromRemote(remote)
	if ip == nil {
		return false
	}
	for _, n := range s.cidrs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func ipFromRemote(remote string) net.IP {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	return net.ParseIP(host)
}

func (s *Server) authenticate(r *http.Request) (*User, error) {
	unauth := domainerr.Unauthorized("RESTCONF requires HTTP Basic")
	user, pass, ok := r.BasicAuth()
	if !ok {
		return nil, unauth
	}
	provided := []byte(pass)
	dummy := []byte("restconf-dummy-password")
	matched := (*User)(nil)
	for i := range s.users {
		u := &s.users[i]
		if subtle.ConstantTimeCompare([]byte(u.Name), []byte(user)) == 1 {
			stored := bytes.TrimRight(u.Password, "\r\n")
			if len(stored) == 0 || len(provided) == 0 {
				_ = subtle.ConstantTimeCompare(dummy, dummy)
				continue
			}
			if subtle.ConstantTimeCompare(stored, provided) == 1 {
				matched = u
			}
			continue
		}
		_ = subtle.ConstantTimeCompare(dummy, dummy)
	}
	if matched == nil {
		return nil, unauth
	}
	return matched, nil
}
