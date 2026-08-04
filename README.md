# phpflash

**The command-line frontend for [php-esp32](https://github.com/php-baremetal/php-esp32) — create,
configure, build, flash and monitor a PHP-on-ESP32 project without touching ESP-IDF by hand.**

`phpflash` is a single static Go binary. It scaffolds a project, drives the ESP-IDF build, flashes
the board, opens the serial console, and can even identify an unknown board that's plugged in. It
replaces php-esp32's `setup.sh` / `flash.sh` / `monitor.sh` with one consistent tool.

It holds **no hard-coded knowledge** of extensions or hardware. The extensions and their build
flags, the chip families and boards, and which storage and execution modes each supports are all
read from the installed php-esp32 at runtime. Add a board or an extension to php-esp32 and it shows
up in `phpflash` with no change here.

## Highlights

- **One tool, whole workflow** — `init` → `system-setup` → `build` → `flash` → `monitor`.
- **Driven by php-esp32** — extensions, settings, boards and modes come from the firmware repo, so
  `init` only ever offers what the firmware actually implements and the chosen board supports.
- **Isolated, reproducible builds** — each project builds into its own tree; the deterministic flag
  list makes a build independent of whatever a shared build directory held before.
- **Board discovery** — `discover` identifies the connected chip and board, and `--all` actively
  probes a blank board's peripherals to name it.
- **Batteries for real firmware** — embedded storage, static DNS, and full-openssl TLS with
  automatic host-CA provisioning (`update-certs`).

## Requirements

- **Go 1.25+** to build from source (a prebuilt binary needs nothing).
- A POSIX host (Linux or macOS). ESP-IDF and the toolchain are installed by `system-setup`.
- A supported board over USB — see [php-esp32](https://github.com/php-baremetal/php-esp32).

## Install

### From a release (Linux)

Each release ships per-architecture binaries and a `SHA256SUMS`. The `latest/download` URL always
points at the newest release:

```sh
BASE=https://github.com/php-baremetal/flash-tool/releases/latest/download
curl -LO "$BASE/phpflash-linux-amd64"      # or phpflash-linux-arm64
curl -LO "$BASE/SHA256SUMS"
sha256sum --ignore-missing -c SHA256SUMS
chmod +x phpflash-linux-amd64
sudo mv phpflash-linux-amd64 /usr/local/bin/phpflash
phpflash --version
```

To pin a version, replace `latest/download` with `download/<tag>` (e.g. `download/v1.0.0`).

### From source

```sh
go build -o phpflash .
```

Put the resulting `phpflash` on your `PATH`.

## Quick start

```sh
phpflash system-setup            # once: install ESP-IDF and php-esp32
phpflash init my-project         # scaffold a project
cd my-project
$EDITOR project-src/index.php    # write your PHP
phpflash build                   # build the firmware
phpflash flash                   # flash it to the board
phpflash monitor                 # open the serial console
```

## Commands

| Command | What it does |
|---|---|
| `init [dir]` | Scaffold a project (config + `project-src/`). |
| `system-setup` | Install ESP-IDF and php-esp32. |
| `build` | Build the firmware for the project. |
| `flash [-p port]` | Build if needed, then flash the board. |
| `monitor [-p port]` | Open the serial console. |
| `update-certs` | Refresh the TLS CA bundle from the host trust store. |
| `discover [-p port]` | Identify the connected chip/board (`--all` actively probes it). |

Run `phpflash <command> --help` for the full flag list.

### `init`

Scaffolds a new project in `dir` (or the current directory): `php-esp32.config.toml`, a `.gitignore`,
and `project-src/index.php`. Interactive, with a sensible default for every prompt; `--yes` accepts
all defaults, `--force` overwrites an existing config. `project-src/` is the **deployable** — on a
`microsd` project it's what you copy to the card — kept apart from the config and build output.

The prompts: project name → chip family and board → storage and execution type → optional extensions
and their settings → serial port → starter (`hello` or `blink`). The board and extension steps are
read live from the installed php-esp32, offering only the modes the firmware implements *and* the
board supports. If php-esp32 isn't installed yet, those steps are skipped with a note.

Flags: `--name`, `--board`, `--php-esp32-path`, `--yes`, `--force`.

### `system-setup`

Installs the global prerequisites:

1. **ESP-IDF** — clone/checkout at the resolved version, then run its `install.sh`.
2. **php-esp32** — clone/checkout the firmware repo, then run its `scripts/fetch-php.sh`.

Idempotent: an existing checkout is updated to the requested version instead of re-cloned; errors
are attributed to the step that failed.

Flags: `--idf-path`, `--idf-version`, `--php-esp32-path`, `--php-esp32-version`.

### `build`

Reads the config, turns the enabled extensions into a deterministic list of `-D<flag>=ON/OFF`
arguments (from php-esp32's manifest), runs any fetch scripts an enabled extension needs, then drives
ESP-IDF into a per-project build tree:

```
idf.py -B ./build/compiled -DSDKCONFIG=./build/sdkconfig \
       -DBOARD=<board> -DPHP_VERSION=<version> -D<flag>… build
```

On success the flashable images (`php-esp32.bin`, `bootloader.bin`, `partition-table.bin`) are copied
up into `build/`. Because the build tree and `sdkconfig` are per-project, several projects share one
php-esp32 install with isolated, side-by-side builds.

For an `embedded` project it also passes `-DPHP_EMBED_SRC=<project-src>` so php-esp32 packs the PHP
source into a read-only image the board runs without a card. The build is shown as phases with a
progress bar; on failure the full ESP-IDF output is printed.

Flags: `--idf-path`, `--php-esp32-path`.

### `flash` / `monitor`

`flash` builds if needed and flashes (`idf.py flash`); `monitor` opens the serial console
(`idf.py monitor`). The port comes from `-p`, else `[board].port`, else the first serial device that
exists — `/dev/ttyACM*` (the on-board CH343P bridge) before `/dev/ttyUSB*` — else ESP-IDF's
autodetect. Detection only checks that the device node exists; it never opens a port.

### `update-certs`

A full-openssl project with the `tls` setting verifies HTTPS peers against a root-CA bundle shipped
with its source (at `certs_path`, default `certs/ca-bundle.crt`). `build` writes that bundle once but
never overwrites it; `update-certs` refreshes it, re-copying the host trust store
(`[extensions.openssl] certs_source`, or the auto-detected system store) over the old one. Errors if
the project doesn't build the TLS client.

### `discover`

Identifies whatever board is plugged in — **from the board outwards, no project needed**:

```
$ phpflash discover
Serial ports:
> /dev/ttyACM0  (USB 1a86:55d3 "USB Single Serial")

Probing /dev/ttyACM0 (resets the board momentarily)...
Chip:     ESP32-P4 (revision v1.3)
Target:   esp32p4
Flash:    32MB
MAC:      e8:f6:0a:e0:ce:92
Radio:    none (no built-in WiFi/BT; this chip needs a companion for wireless)

Supported boards for esp32p4 (family "ESP32-P4"):
  - esp32-p4-eth    ESP32-P4-ETH     network: ethernet  microSD: yes (-DBOARD=esp32-p4-eth)
  - esp32-p4-pico   ESP32-P4-Pico    network: none      microSD: yes (-DBOARD=esp32-p4-pico)
```

- **Chip** and **built-in WiFi/BT** come straight from the silicon (esptool + the chip's features).
- **LAN (Ethernet) and microSD** are board-level, so the chip can't be probed for them; `discover`
  reports what each candidate **board model** declares in its `board.toml`.
- If the chip matches no board here, it says so plainly (an **unsupported board**) and points you at
  `boards/<family>/<board>/` to add one.

`--no-probe` lists ports + USB only (no reset). `--idf-path` / `--php-esp32-path` override locations.

#### `discover --all` — actively probe the peripherals

For a **blank board** the chip alone can't tell, say, a P4-ETH from a P4-Pico. `--all` settles it by
flashing a small **discovery firmware**, built **per candidate board, reusing that board's
`board.c`** (so it uses the real GPIO wiring — no hardcoded pins; a new board's probe follows
automatically). It brings up each board's peripherals and names the match:

```
$ phpflash discover --all
...
== Probing with the esp32-p4-eth wiring ==
  board wiring: ESP32-P4-ETH
  Ethernet:     up (10.42.0.224)
  microSD:      card present (14895MB)
  PSRAM:        32MB

=> this board is: ESP32-P4-ETH  (build with -DBOARD=esp32-p4-eth)
```

It is **destructive** — it overwrites the app on the board — so it warns, asks first (`-y`/`--yes`
skips the prompt), and reminds you to re-flash afterwards. Ethernet detection needs the cable
connected (it brings the link up).

## Configuration

`init` writes `php-esp32.config.toml`; the other commands read it.

```toml
name         = "my-project"
storage_type = "microsd"      # where the PHP source lives (microsd | embedded)
type         = "init-loop"    # execution model (init-loop | web-server | event-driven)

[board]
target = "esp32-p4-pico"      # the board (its family decides the ESP-IDF target)
port   = ""                   # serial port; empty = autodetect at flash time

# Toolchain and firmware sources. An empty path/version means "use the default".
[esp-idf]
path    = ""                  # e.g. ~/esp/esp-idf
version = "v5.5.5"            # git ref

[php-esp32]
path    = ""
version = ""                  # git ref; empty = default branch

# Only for storage_type = "embedded": also mount a microSD for writable data (off by default).
[storage]
microsd = false

# Static DNS for a networked board (optional; DHCP-provided servers are used otherwise).
[network]
dns = ["1.1.1.1", "8.8.8.8"]

# One table per enabled extension: `enabled` plus any settings it declares (bool or string).
[extensions.sqlite]
enabled = true

[extensions.openssl]
enabled = true
full    = true                # the real OpenSSL 3.0 (RSA/EC/X.509)
tls     = true                # HTTPS client (needs a networked board)

[php]
src   = "project-src"         # the PHP source folder (copied to the SD / embedded)
entry = "index.php"           # entry file within src
```

### Local overrides — `php-esp32.config.local.toml`

If a `php-esp32.config.local.toml` sits next to the config, every command overlays it: keys it sets
win, everything else keeps the committed value. It's for machine-specific tweaks (a serial port, a
toolchain path). It's optional, `init` never creates it, and the scaffolded `.gitignore` lists it, so
it stays out of version control.

### Path and version resolution

Highest precedence wins:

1. **CLI flag** — `--idf-path`, `--php-esp32-path`, `--idf-version`, `--php-esp32-version`
2. **`php-esp32.config.toml`** — `path` / `version`
3. **Environment** — `IDF_PATH`, `PHP_ESP32_DIR`
4. **Default** — `~/esp/esp-idf`, `~/esp/php-esp32`

## How it works

Everything about extensions and hardware is owned by php-esp32 and read at runtime:

- `php-esp32.toml` (repo root) — the default PHP version and board;
- `components/php/versions/<version>/manifest.toml` — the per-version extension manifest (flags,
  settings, `requires`, `required_for`, fetch scripts, and the modes the firmware implements);
- `boards/<family>/<board>/board.toml` and `boards/<family>/family.toml` — board and chip-family
  descriptors.

A project's build flags are derived entirely from the manifest: the project type's mandatory
extensions, plus the enabled optional ones (with `requires` pulled in transitively) and their
settings, become `-D<flag>=ON`; every other optional flag is emitted `=OFF`. The full architecture is
in [docs/design.md](docs/design.md).

## Development

```sh
go test ./...
go vet ./...
go build -o phpflash .
```

The parts worth testing are pure logic behind interfaces, so they run without a real toolchain,
board or network (config round-trips, manifest resolution, the exact `idf.py` argument list, the
discovery parsers). Real `idf.py`, `git`, and serial I/O are integration concerns.

## Changelog & license

- Release history: [CHANGELOG.md](CHANGELOG.md).
- Licensed under the [MIT License](LICENSE).
