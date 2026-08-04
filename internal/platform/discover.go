package platform

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListPorts returns every serial device matching the known patterns (all /dev/ttyACM* first,
// then all /dev/ttyUSB*), each pattern's matches sorted. Like DetectPort it only checks that the
// device node exists -- it never opens a port. Empty if nothing is plugged in.
func ListPorts() []string { return listPorts(serialGlobs) }

func listPorts(globs []string) []string {
	var out []string
	for _, g := range globs {
		m, _ := filepath.Glob(g)
		sort.Strings(m)
		out = append(out, m...)
	}
	return out
}

// USBInfo is the USB identity of a serial device, read from the descriptors -- enough to name the
// USB-serial bridge (e.g. a CH343P) without opening or resetting anything.
type USBInfo struct {
	Vendor       string // idVendor, e.g. "1a86"
	Product      string // idProduct, e.g. "55d3"
	Manufacturer string // manufacturer string, e.g. "wch.cn"
	ProductName  string // product string, e.g. "USB Single Serial"
}

// PortUSB reads the USB descriptors for a serial device from Linux sysfs (best-effort). It walks
// up from /sys/class/tty/<dev>/device to the USB device node that carries idVendor, and returns
// its vendor/product ids and strings. ok is false off Linux or when the info isn't available --
// callers treat that as "unknown", not an error.
func PortUSB(port string) (USBInfo, bool) {
	dev := filepath.Base(port) // ttyACM0
	start, err := filepath.EvalSymlinks("/sys/class/tty/" + dev + "/device")
	if err != nil {
		return USBInfo{}, false
	}
	for d := start; d != "/" && d != "." && d != ""; d = filepath.Dir(d) {
		vid, err := readSysAttr(d, "idVendor")
		if err != nil {
			continue
		}
		return USBInfo{
			Vendor:       vid,
			Product:      mustAttr(d, "idProduct"),
			Manufacturer: mustAttr(d, "manufacturer"),
			ProductName:  mustAttr(d, "product"),
		}, true
	}
	return USBInfo{}, false
}

func readSysAttr(dir, name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func mustAttr(dir, name string) string {
	s, _ := readSysAttr(dir, name)
	return s
}
