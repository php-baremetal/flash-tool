package build

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
)

// serialReaderPy streams a board's serial output to stdout for a bounded time, non-interactively.
// It keeps the modem lines in the "run" state (DTR/RTS deasserted) so opening the port never drops
// the chip into download, and reconnects across the USB re-enumeration a native-USB board does when
// it resets -- so a one-shot boot banner isn't lost. No single quotes (bash single-quotes the whole
// script). argv: port, duration_s, until-marker (empty = none), reset ("1"/"0").
const serialReaderPy = `
import serial,time,sys
port=sys.argv[1]; dur=float(sys.argv[2]); mark=sys.argv[3].encode(); do_reset=sys.argv[4]=="1"
deadline=time.time()+dur; tail=b""; first=True
while time.time()<deadline:
 try:
  p=serial.Serial(); p.port=port; p.baudrate=115200; p.timeout=0.2
  p.dtr=False; p.rts=False
  p.open()
  if first and do_reset:
   p.rts=True; time.sleep(0.12); p.rts=False
 except Exception:
  time.sleep(0.2); continue
 first=False
 try:
  while time.time()<deadline:
   d=p.read(512)
   if d:
    sys.stdout.buffer.write(d); sys.stdout.flush()
    if mark:
     tail=(tail+d)[-4096:]
     if mark in tail:
      deadline=min(deadline,time.time()+0.3)
 except Exception:
  pass
 finally:
  try: p.close()
  except Exception: pass
 time.sleep(0.1)
`

// SerialRead streams the board's serial output to out for durationSec, or until `until` appears
// (whichever comes first), then returns -- a non-interactive monitor that needs no TTY, for scripts
// and CI. It uses the pyserial that ships with ESP-IDF (no extra dependency). With reset it pulses a
// reset-to-run first, to catch the boot output; without it, it tails whatever is already running.
func SerialRead(idfPath, port string, durationSec int, until string, reset bool, out io.Writer) error {
	if port == "" {
		return fmt.Errorf("no serial port: pass -p or set [esp32] port")
	}
	r := "0"
	if reset {
		r = "1"
	}
	script := ". " + shquote(filepath.Join(idfPath, "export.sh")) +
		" >/dev/null 2>&1 && python -c " + shquote(serialReaderPy) +
		" " + shquote(port) + " " + shquote(strconv.Itoa(durationSec)) +
		" " + shquote(until) + " " + shquote(r)
	c := exec.Command("bash", "-c", script)
	c.Stdout, c.Stderr = out, out
	return c.Run()
}
