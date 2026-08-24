package discover

import (
	"testing"

	"phpflash/internal/manifest"
)

func TestTargetFromChip(t *testing.T) {
	cases := map[string]string{
		"ESP32-P4": "esp32p4",
		"ESP32-S3": "esp32s3",
		"ESP32":    "esp32",
		"ESP32-C6": "esp32c6",
		" ESP32-H2 ": "esp32h2",
		"ESP32-S3 (QFN56)": "esp32s3",
		"ESP32-P4 (QFN40)": "esp32p4",
	}
	for in, want := range cases {
		if got := TargetFromChip(in); got != want {
			t.Errorf("TargetFromChip(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseChipInfo(t *testing.T) {
	out := `esptool.py v4.12.0
Serial port /dev/ttyACM0
Connecting....
Chip is ESP32-P4 (revision v1.3)
Features: High-Performance MCU
Crystal is 40MHz
MAC: e8:f6:0a:e0:ce:92
Uploading stub...
Stub running...
Manufacturer: 0d
Device: 4017
Detected flash size: 32MB
Hard resetting via RTS pin...`
	c := ParseChipInfo(out)
	if c.Chip != "ESP32-P4" || c.Target != "esp32p4" || c.Revision != "v1.3" {
		t.Errorf("chip/target/rev = %q/%q/%q", c.Chip, c.Target, c.Revision)
	}
	if c.MAC != "e8:f6:0a:e0:ce:92" || c.Flash != "32MB" || c.Crystal != "40MHz" {
		t.Errorf("mac/flash/crystal = %q/%q/%q", c.MAC, c.Flash, c.Crystal)
	}
	if c.Features != "High-Performance MCU" {
		t.Errorf("features = %q", c.Features)
	}
}

func TestParseChipInfoNoRevision(t *testing.T) {
	c := ParseChipInfo("Chip is ESP32\nMAC: aa:bb:cc:dd:ee:ff\n")
	if c.Chip != "ESP32" || c.Target != "esp32" || c.Revision != "" {
		t.Errorf("got %q / %q / rev=%q", c.Chip, c.Target, c.Revision)
	}
}

func TestRadioFromFeatures(t *testing.T) {
	cases := map[string]string{
		"High-Performance MCU":       "none",     // ESP32-P4: no radio
		"WiFi, BLE":                  "WiFi, Bluetooth",
		"WiFi 6, BT 5, IEEE802.15.4": "WiFi, Bluetooth, 802.15.4",
		"WiFi, BT, Dual Core, 240MHz, Embedded Flash": "WiFi, Bluetooth",
		"":                           "none",
	}
	for in, want := range cases {
		if got := RadioFromFeatures(in); got != want {
			t.Errorf("RadioFromFeatures(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseDiscoverFW(t *testing.T) {
	// an ETH board: network up, card present, with a warning line interleaved as really emitted
	raw := `DISCOVER-FW-BEGIN
board=ESP32-P4-ETH
chip=ESP32-P4
cores=2
revision=103
psram=32MB
mac=e8:f6:0a:e0:ce:92
ethernet=yes
ip=10.42.0.224
W (1839) ldo: The voltage value 0 is out of the recommended range
microsd=card:14910MB
DISCOVER-FW-END`
	d := ParseDiscoverFW(raw)
	if !d.Seen || d.Board != "ESP32-P4-ETH" || d.Chip != "ESP32-P4" || d.PSRAM != "32MB" {
		t.Errorf("basic fields: %+v", d)
	}
	if !d.Ethernet || d.EthernetNA || d.IP != "10.42.0.224" {
		t.Errorf("ethernet = %v / na=%v / ip=%q", d.Ethernet, d.EthernetNA, d.IP)
	}
	if !d.MicroSD || d.CardSize != "14910MB" {
		t.Errorf("microsd = %v / %q", d.MicroSD, d.CardSize)
	}

	// a Pico-like board: no network hardware (n/a), no card
	d2 := ParseDiscoverFW("DISCOVER-FW-BEGIN\nboard=ESP32-P4-Pico\nethernet=n/a\nmicrosd=nocard\nDISCOVER-FW-END")
	if d2.Ethernet || !d2.EthernetNA || d2.MicroSD {
		t.Errorf("expected n/a ethernet + no card, got %+v", d2)
	}

	// a Zero-like board: no network hardware AND no card slot (both n/a)
	d3 := ParseDiscoverFW("DISCOVER-FW-BEGIN\nboard=ESP32-P4-Zero\nethernet=n/a\nmicrosd=n/a\nDISCOVER-FW-END")
	if d3.MicroSD || !d3.MicroSDNA || !d3.EthernetNA {
		t.Errorf("expected n/a ethernet + n/a microsd (no slot), got %+v", d3)
	}

	// no block at all
	if ParseDiscoverFW("random noise").Seen {
		t.Errorf("Seen should be false without the block")
	}
}

func TestFamilyForTarget(t *testing.T) {
	fams := []manifest.Family{
		{Key: "esp32-p4", Name: "ESP32-P4", Target: "esp32p4"},
		{Key: "esp32-s3", Name: "ESP32-S3", Target: "esp32s3"},
	}
	if f := FamilyForTarget(fams, "esp32p4"); f == nil || f.Key != "esp32-p4" {
		t.Errorf("supported chip: got %v", f)
	}
	// an unsupported board: chip is real, but no family here covers it
	if f := FamilyForTarget(fams, "esp32c6"); f != nil {
		t.Errorf("unsupported chip should return nil, got %v", f)
	}
}
