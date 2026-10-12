package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChipPsramMB(t *testing.T) {
	cases := map[string]int{
		"WiFi, BLE, Embedded Flash 4MB (XMC), Embedded PSRAM 2MB (AP_3v3)": 2,
		"WiFi, BLE, Embedded PSRAM 8MB (AP_3v3)":                           8,
		"WiFi, BLE, Embedded Flash 16MB":                                   0, // no PSRAM
		"High-Performance MCU":                                             0,
	}
	for in, want := range cases {
		if got := chipPsramMB(in); got != want {
			t.Errorf("chipPsramMB(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestPsramSuspect(t *testing.T) {
	cases := []struct {
		want    string
		mb      int
		suspect bool
		guess   string
	}{
		{"octal", 8, false, ""},    // consistent
		{"octal", 2, true, "quad"}, // 2 MB can't be octal
		{"quad", 2, false, ""},     // consistent
		{"quad", 8, true, "octal"}, // 8 MB might be octal
	}
	for _, c := range cases {
		s, g := psramSuspect(c.want, c.mb)
		if s != c.suspect || g != c.guess {
			t.Errorf("psramSuspect(%q,%d) = (%v,%q), want (%v,%q)", c.want, c.mb, s, g, c.suspect, c.guess)
		}
	}
}

func TestExpectedPsramMode(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "sdkconfig")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if m := expectedPsramMode(write("CONFIG_SPIRAM=y\nCONFIG_SPIRAM_MODE_OCT=y\n")); m != "octal" {
		t.Errorf("octal sdkconfig -> %q", m)
	}
	if m := expectedPsramMode(write("CONFIG_SPIRAM_MODE_QUAD=y\n")); m != "quad" {
		t.Errorf("quad sdkconfig -> %q", m)
	}
	if m := expectedPsramMode(write("CONFIG_SPIRAM=y\n")); m != "" {
		t.Errorf("no-mode sdkconfig -> %q (want empty)", m)
	}
	if m := expectedPsramMode(filepath.Join(dir, "nope")); m != "" {
		t.Errorf("missing sdkconfig -> %q (want empty)", m)
	}
}
