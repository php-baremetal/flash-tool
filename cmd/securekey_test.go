package cmd

import "testing"

func TestNormalizeMAC(t *testing.T) {
	cases := map[string]string{
		"28:84:85:67:57:80": "288485675780",
		"28-84-85-67-57-80": "288485675780",
		"288485675780":      "288485675780",
		"AA:BB:CC:DD:EE:FF": "aabbccddeeff",
	}
	for in, want := range cases {
		if got := normalizeMAC(in); got != want {
			t.Errorf("normalizeMAC(%q) = %q, want %q", in, got, want)
		}
	}
}
