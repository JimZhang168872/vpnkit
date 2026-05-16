package portutil

import (
	"fmt"
	"net"
	"testing"
)

func TestFindFreeReturnsStartWhenAvailable(t *testing.T) {
	// Pick a high random port by binding :0, close it, then call FindFree at that port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	start := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	got, err := FindFree(start, 10)
	if err != nil {
		t.Fatalf("FindFree: %v", err)
	}
	if got != start {
		t.Errorf("got %d, want %d (start)", got, start)
	}
}

func TestFindFreeSkipsOccupied(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	start := ln.Addr().(*net.TCPAddr).Port

	got, err := FindFree(start, 50)
	if err != nil {
		t.Fatalf("FindFree: %v", err)
	}
	if got == start {
		t.Errorf("FindFree returned occupied port %d", got)
	}
	if got < start || got > start+50 {
		t.Errorf("FindFree returned %d, out of range [%d,%d]", got, start, start+50)
	}
}

func TestFindFreeErrorsWhenAllOccupied(t *testing.T) {
	// Occupy span+1 consecutive ports (the entire FindFree search window).
	const span = 3
	const need = span + 1
	var lns []net.Listener
	defer func() {
		for _, l := range lns {
			_ = l.Close()
		}
	}()
	for attempt := 0; attempt < 200; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		start := ln.Addr().(*net.TCPAddr).Port
		lns = []net.Listener{ln}
		ok := true
		for i := 1; i < need; i++ {
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", start+i))
			if err != nil {
				for _, x := range lns {
					_ = x.Close()
				}
				lns = nil
				ok = false
				break
			}
			lns = append(lns, l)
		}
		if !ok {
			continue
		}
		if _, err := FindFree(start, span); err == nil {
			t.Fatalf("FindFree should have errored when window full")
		}
		return
	}
	t.Skip("could not allocate consecutive free ports")
}

func TestIsFreeOnLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if IsFree(port) {
		t.Errorf("port %d busy but IsFree=true", port)
	}
	_ = ln.Close()
	if !IsFree(port) {
		t.Errorf("port %d free but IsFree=false", port)
	}
}
