package main

import "testing"

func TestPrivateAddr(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:8767":           true,
		"localhost:8767":           true,
		"[::1]:8767":               true,
		"100.83.24.70:8767":        true, // Tailscale
		"192.168.1.5:8767":         true,
		"10.0.0.2:8767":            true,
		"[fd7a:115c:a1e0::1]:8767": true,
		"0.0.0.0:8767":             false,
		":8767":                    false,
		"[::]:8767":                false,
		"8.8.8.8:8767":             false,
		"100.128.0.1:8767":         false, // just past the tailnet range
		"example.com:8767":         false,
	} {
		if err := privateAddr(addr); (err == nil) != ok {
			t.Errorf("%s: %v, want ok=%v", addr, err, ok)
		}
	}
}
