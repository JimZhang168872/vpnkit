// Package portutil contains helpers for picking a free TCP port on 127.0.0.1.
//
// vpnkit needs this because mihomo's mixed-port and external-controller default
// to 7890/9090; on multi-user hosts two vpnkit installs would otherwise collide.
package portutil

import (
	"fmt"
	"net"
)

// IsFree returns true if a TCP listener can bind 127.0.0.1:port right now.
func IsFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// FindFree scans [start, start+span] on 127.0.0.1 and returns the first free port.
// Span 0 means "try only start". Returns an error if every candidate is occupied.
func FindFree(start, span int) (int, error) {
	if start <= 0 || start > 65535 {
		return 0, fmt.Errorf("portutil: invalid start %d", start)
	}
	if span < 0 {
		span = 0
	}
	for i := 0; i <= span; i++ {
		p := start + i
		if p > 65535 {
			break
		}
		if IsFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("portutil: no free port in [%d, %d]", start, start+span)
}
