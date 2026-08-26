---
eyebrow: 'Docs · Commands'
lede: 'Open the serial console on the board over the same USB cable you flashed with. It wraps idf.py monitor, so backtraces are decoded for you; the boot log shows the board coming up, mounting its card, and — on a networked board — the address it is reachable on.'
see_also:
  - { href: './flash.md', meta: 'Commands', label: 'phpflash flash' }
  - { href: './discover.md', meta: 'Commands', label: 'phpflash discover' }
  - { href: '../../getting-started/quick-start.md', meta: '10 min', label: 'Quick start' }
prev: { label: 'phpflash flash', href: './flash.md' }
next: { label: 'phpflash discover', href: './discover.md' }
---

# `phpflash monitor`

`phpflash monitor` opens the serial console on the connected board, running `idf.py monitor` against
the project's own build tree. It needs only a port and the tool paths — no manifest, no build args — so
it starts fast. A project config, if present, supplies default path and port values, but `monitor`
works without one.

<!-- @code-block language="bash" label="terminal — monitor" -->
```bash
phpflash monitor            # autodetect the port
phpflash monitor -p /dev/ttyACM0
```
<!-- @endcode-block -->

## Flags

| Flag | Meaning |
|---|---|
| `-p, --port <port>` | Serial port. Empty tries `/dev/ttyACM*`, then `/dev/ttyUSB*`, then lets ESP-IDF autodetect. |
| `--idf-path <dir>` | ESP-IDF location (overrides config, env and default). |
| `--php-esp32-path <dir>` | php-esp32 location. |

The port is resolved just as it is for [`flash`](./flash.md): the `-p` flag, then the config's
`[board].port`, then the first serial device that exists (`/dev/ttyACM*` before `/dev/ttyUSB*`), else
empty so ESP-IDF autodetects.

## Leaving the monitor

The console runs until you exit it:

<!-- @steps -->
- **`Ctrl-]`** — leave the monitor and return to the shell.
- **`Ctrl-T` `Ctrl-R`** — reset the board without reflashing, so you can watch it boot again from the
  top.
<!-- @endsteps -->

<!-- @callout variant="note" title="Backtraces are decoded for you" -->
Because `monitor` wraps `idf.py monitor`, a crash backtrace printed by the firmware is decoded to
file-and-line against the build's ELF automatically — you do not run a separate address decoder. A
board that keeps resetting with an out-of-memory backtrace usually means the app needs more PSRAM than
the board has.
<!-- @endcallout -->

## What the boot log shows

The serial output is where the firmware reports what it is doing. What appears depends on the board and
the project, but the useful landmarks are:

<!-- @code-block language="text" label="Serial console at boot (networked web-server board)" -->
```
microSD mounted at /sdcard
opcache: file cache at /sdcard/opcache
network up -- http://10.42.0.224/
web-server model: serving /app/index.php over HTTP on :80
```
<!-- @endcode-block -->

- **`microSD mounted at /sdcard`** confirms the card was found and mounted; a `microsd` project reads
  its source from there.
- **`network up -- http://<ip>/`** prints on a networked board the address it came up on. That is where
  you point a browser or `curl` for a `web-server` project.
- An `init-loop` project's `echo` output — a linear script's lines, or the `setup()`/`loop()` ticks —
  streams here as it runs.

<!-- @callout variant="warning" title="No serial port" -->
If `monitor` finds nothing to open, check `ls /dev/ttyACM* /dev/ttyUSB*`. An absent port usually means
the USB cable carries power only, not data. A busy port can be `ModemManager` probing the device on
some distributions; add yourself to the `dialout` group, or free the port, and retry.
<!-- @endcallout -->
