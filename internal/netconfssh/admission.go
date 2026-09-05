package netconfssh

import (
	"net"
	"net/netip"
)

var loopbackCIDRs = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
}

// admit reports whether remote is in cidrs. A nil list is loopback;
// a non-nil empty list is deny-all.
func admit(remote net.Addr, cidrs []netip.Prefix) bool {
	if cidrs == nil {
		cidrs = loopbackCIDRs
	}
	ip := addrIP(remote)
	if !ip.IsValid() {
		return false
	}
	for _, p := range cidrs {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func addrIP(remote net.Addr) netip.Addr {
	if remote == nil {
		return netip.Addr{}
	}
	if tcp, ok := remote.(*net.TCPAddr); ok && tcp.IP != nil {
		ip, ok := netip.AddrFromSlice(tcp.IP)
		if !ok {
			return netip.Addr{}
		}
		return ip.Unmap()
	}
	host, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		host = remote.String()
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}

// ParseCIDRs parses admission CIDR strings.
func ParseCIDRs(cidrs []string) ([]netip.Prefix, error) {
	if cidrs == nil {
		return nil, nil
	}
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, s := range cidrs {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
