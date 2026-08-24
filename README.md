<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/logo-light.svg">
    <img alt="phpflash" src="assets/logo-light.svg" width="440">
  </picture>
</div>

<p align="center">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/php-baremetal/flash-tool?style=flat-square&color=475569"></a>
  <img alt="Go 1.25+" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="Host: Linux | macOS" src="https://img.shields.io/badge/host-Linux%20%7C%20macOS-2EA44F?style=flat-square&logo=linux&logoColor=white">
  <img alt="Single static binary" src="https://img.shields.io/badge/single-static%20binary-475569?style=flat-square">
  <a href="https://github.com/php-baremetal/flash-tool/stargazers"><img alt="Stars" src="https://img.shields.io/github/stars/php-baremetal/flash-tool?style=flat-square&color=475569"></a>
</p>

# phpflash

The command-line front end for [php-esp32](https://github.com/php-baremetal/php-esp32): create,
configure, build, flash and monitor a PHP-on-ESP32 project without touching ESP-IDF by hand.

`phpflash` is a single static Go binary. It scaffolds a project, drives the ESP-IDF build, flashes the
board, opens the serial console, and can identify an unknown board that is plugged in. It replaces
php-esp32's `setup.sh`, `flash.sh` and `monitor.sh` with one consistent tool.

It holds no hard-coded knowledge of extensions or hardware. The extensions and their build flags, the
chip families and boards, and which storage and execution modes each supports are all read from the
installed php-esp32 at runtime. Add a board, an extension or a PHP version to php-esp32 and it appears
in phpflash with no change here. Two chip families are supported today (ESP32-P4 and ESP32-S3) and PHP
8.3, 8.4 and 8.5; as php-esp32 grows, so does what phpflash offers.

## Highlights

- **One tool, whole workflow.** `init`, then `system-setup`, `build`, `flash`, `monitor`.
- **Driven by php-esp32.** Extensions, settings, boards and modes come from the firmware repo, so
  `init` only ever offers what the firmware implements and the chosen board supports.
- **Isolated, reproducible builds.** Each project builds into its own tree; a deterministic flag list
  makes a build independent of whatever a shared build directory held before, and pins the ESP-IDF
  target from the board's family so the wrong architecture cannot be built by accident.
- **Board discovery.** `discover` identifies the connected chip and board, and `--all` actively probes
  a blank board's peripherals to name it.
- **Guardrails.** A flash checks that the connected chip matches the project's board before writing,
  and a microSD flash clears any leftover embedded image so it cannot shadow the card.
- **Batteries for real firmware.** Embedded storage, static DNS, and full-openssl TLS with automatic
  host-CA provisioning (`update-certs`).
- **Per-project, from the config.** Pin the PHP version, scaffold a custom C extension (`ext new`),
  bake a `.env` into the firmware as `$_ENV` / `getenv()`, and size a reboot-persistent key-value
  store -- each set in `php-esp32.config.toml`, no firmware fork.

## Requirements

- **Go 1.25+** to build from source (a prebuilt binary needs nothing).
- A POSIX host (Linux or macOS). ESP-IDF and the toolchain are installed by `system-setup`.
- A supported board over USB. See [php-esp32](https://github.com/php-baremetal/php-esp32).

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

To pin a version, replace `latest/download` with `download/<tag>` (for example `download/v1.0.0`).

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
| `init [dir]` | Scaffold a project (config plus `project-src/`). |
| `system-setup` | Install ESP-IDF and php-esp32. |
| `build` | Build the firmware for the project. |
| `flash [-p port]` | Build if needed, check the board, then flash. |
| `monitor [-p port]` | Open the serial console. |
| `update-certs` | Refresh the TLS CA bundle from the host trust store. |
| `discover [-p port]` | Identify the connected chip and board (`--all` actively probes it). |
| `ext new <name>` | Scaffold a custom C extension under `firmware/exts/<name>/`. |

Run `phpflash <command> --help` for the full flag list.

The per-command detail (every flag, and what each step does) is in
[docs/commands.md](docs/commands.md). A few notes worth having up front:

- `init` reads the installed php-esp32, so its board and extension prompts offer only what the firmware
  implements and the chosen board supports. `--yes` takes every default.
- `build` produces an isolated per-project build tree and pins the ESP-IDF target from the board's
  family, so a stray in-source `sdkconfig` cannot build the wrong architecture.
- `flash` checks the connected chip against the project board before writing (`--force` to override)
  and clears a leftover embedded image after a `microsd` flash, so it cannot shadow the card.

### `discover`

Identifies whatever board is plugged in, from the board outwards, with no project needed:

```
$ phpflash discover
Serial ports:
> /dev/ttyACM0  (USB 303a:1001 "USB JTAG/serial debug unit")

Probing /dev/ttyACM0 (resets the board momentarily)...
Chip:     ESP32-S3 (QFN56) (revision v0.2)
Target:   esp32s3
Flash:    16MB
MAC:      28:84:85:54:d0:50
Radio:    WiFi, Bluetooth (built into the chip)

Supported boards for esp32s3 (family "ESP32-S3"):
  - esp32-s3-eth    ESP32-S3-ETH     network: ethernet  microSD: yes (-DBOARD=esp32-s3-eth)

To target one from a project, set in php-esp32.config.toml:
  [board]
  target = "esp32-s3-eth"
```

- **Chip** and **built-in WiFi/BT** come straight from the silicon (esptool plus the chip's features).
- **LAN (Ethernet) and microSD** are board-level, so the chip cannot be probed for them; `discover`
  reports what each candidate board model declares in its `board.toml`.
- If the chip matches no board here, it says so plainly (an unsupported board) and points you at
  `boards/<family>/<board>/` to add one.

`--no-probe` lists ports and USB only (no reset). `--idf-path` and `--php-esp32-path` override
locations.

#### `discover --all`: actively probe the peripherals

For a blank board the chip alone cannot tell, say, a P4-ETH from a P4-Pico. `--all` settles it by
flashing a small discovery firmware, built per candidate board, reusing that board's `board.c` so it
uses the real GPIO wiring (no hardcoded pins; a new board's probe follows automatically). It brings up
each board's peripherals and names the match:

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

It is destructive: it overwrites the app on the board, so it warns, asks first (`-y`/`--yes` skips the
prompt), and reminds you to re-flash afterward. Ethernet detection needs the cable connected, since it
brings the link up.

## Configuration

`init` writes `php-esp32.config.toml`; the other commands read it. A short one looks like this:

```toml
name         = "my-project"
storage_type = "microsd"      # microsd | embedded
type         = "init-loop"    # init-loop | web-server | event-driven

[board]
target = "esp32-p4-pico"      # its family decides the ESP-IDF target
port   = ""                   # empty = autodetect at flash time

[extensions.mbstring]
enabled = true
onig    = true

[php]
src     = "project-src"
entry   = "index.php"
version = ""                  # empty = the firmware's default PHP version
```

Beyond these, a project can bake a `.env` into the firmware (read as `$_ENV` / `getenv()`), size a
reboot-persistent key-value store with `[store] size_kb`, and ship custom C extensions in
`firmware/exts/` (see `ext new`). Every key, table and extension setting, with its type and default,
is documented in [docs/configuration.md](docs/configuration.md). A sibling
`php-esp32.config.local.toml`, if present, is overlaid on top for machine-specific tweaks (a serial
port, a toolchain path) and is git-ignored.

## Documentation

- [docs/configuration.md](docs/configuration.md): every option in `php-esp32.config.toml`.
- [docs/commands.md](docs/commands.md): the full command and flag reference.
- [docs/design.md](docs/design.md): how phpflash is built and how it reads php-esp32.

For the firmware itself (writing PHP for the board, the extensions, the porting details), see the
[php-esp32](https://github.com/php-baremetal/php-esp32) docs.

## How it works

Everything about extensions and hardware is owned by php-esp32 and read at runtime:

- `php-esp32.toml` (repo root): the default PHP version and board.
- `components/php/versions/<version>/manifest.toml`: the per-version extension manifest (flags,
  settings, `requires`, `required_for`, fetch scripts, and the modes the firmware implements).
- `boards/<family>/<board>/board.toml` and `boards/<family>/family.toml`: the board and chip-family
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

The parts worth testing are pure logic behind interfaces, so they run without a real toolchain, board
or network (config round-trips, manifest resolution, the exact `idf.py` argument list, the discovery
parsers). Real `idf.py`, `git`, and serial I/O are integration concerns.

## Changelog and license

- Release history: [CHANGELOG.md](CHANGELOG.md).
- Licensed under the [MIT License](LICENSE).
