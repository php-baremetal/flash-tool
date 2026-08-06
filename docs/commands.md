# Command reference

Every phpflash command, its flags and what it does. `phpflash <command> --help` prints the same flag
list at the terminal.

The usual order is `system-setup` once, then `init` per project, then `build`, `flash` and `monitor`.
`discover` and `update-certs` are used as needed.

## `init [dir]`

Scaffold a new project in `dir`, or the current directory if omitted. Writes three things:
`php-esp32.config.toml`, a `.gitignore`, and `project-src/index.php`. `project-src/` is the deployable,
kept apart from the config and the build output.

It is interactive, with a default at every step, and it reads the installed php-esp32 so it only
offers what the firmware can build. The prompts, in order: project name, chip family, board, storage
type, execution type, optional extensions and their settings, serial port, and a `hello` or `blink`
starter. If php-esp32 is not installed, the board and extension steps are skipped with a note, and you
fill those in after `system-setup`.

| Flag | Meaning |
|---|---|
| `--name <name>` | Project name (default: the directory name). |
| `--board <board>` | Preselect a board, for example `esp32-s3-eth`. |
| `--php-esp32-path <dir>` | Where php-esp32 is installed (to read boards and extensions from). |
| `--yes` | Accept every default and ask nothing. |
| `--force` | Overwrite an existing `php-esp32.config.toml`. |

```sh
phpflash init my-project                       # interactive
phpflash init my-project --board esp32-s3-eth  # preselect the board
phpflash init . --yes                          # scaffold here with all defaults
```

## `system-setup`

Install the global prerequisites, once per machine. It is idempotent: an existing checkout is updated
to the requested version rather than re-cloned, and an error is attributed to the step that failed.

1. **ESP-IDF**: clone or check out at the resolved version, then run its `install.sh` (which brings the
   cross-compilers and a private Python environment).
2. **php-esp32**: clone or check out the firmware repo, then run its `scripts/fetch-php.sh` to download
   and patch the PHP source.

| Flag | Meaning |
|---|---|
| `--idf-path <dir>` | Where to install ESP-IDF (default `~/esp/esp-idf`). |
| `--idf-version <ref>` | The ESP-IDF git ref to check out. |
| `--php-esp32-path <dir>` | Where to install php-esp32 (default `~/esp/php-esp32`). |
| `--php-esp32-version <ref>` | The php-esp32 git ref to check out. |

## `build`

Read the project config, turn the enabled extensions into a deterministic `-D<flag>=ON/OFF` list from
the manifest, run any fetch scripts an enabled extension needs, then drive ESP-IDF into a per-project
build tree under `build/`:

```
idf.py -B ./build/compiled -DSDKCONFIG=./build/sdkconfig \
       -DBOARD=<board> -DPHP_VERSION=<version> -DIDF_TARGET=<target> -D<flag>... build
```

On success the flashable images (`php-esp32.bin`, `bootloader.bin`, `partition-table.bin`) are copied
up into `build/`. The build tree and `sdkconfig` are per project, so several projects share one
php-esp32 install with isolated builds. The target is pinned from the board's family, so ESP-IDF cannot
guess a different architecture from a stray in-source `sdkconfig`. An `embedded` project also gets
`-DPHP_EMBED_SRC=<project-src>`. Progress is shown as phases with a bar; on failure the full ESP-IDF
output is printed.

| Flag | Meaning |
|---|---|
| `--idf-path <dir>` | ESP-IDF location (overrides config, env and default). |
| `--php-esp32-path <dir>` | php-esp32 location. |

## `flash`

Build if needed, then flash the board. Two guardrails run around the write:

- **Board check.** phpflash probes the connected chip and refuses if its target does not match the
  project board's family (an S3 image with a P4 plugged in, say), with a message naming both. `--force`
  flashes anyway. A probe that cannot run does not block the flash, since esptool verifies the chip
  during the write.
- **Storage cleanup.** After a `microsd` flash it erases the board's `storage` partition, so a leftover
  embedded image from an earlier build cannot mount and shadow the card.

The port comes from `-p`, then the config's `[board].port`, then the first serial device that exists.

| Flag | Meaning |
|---|---|
| `-p, --port <port>` | Serial port (default: `/dev/ttyACM*`, then autodetect). |
| `--force` | Flash even if the connected chip does not match the project's board. |
| `--idf-path <dir>` | ESP-IDF location. |
| `--php-esp32-path <dir>` | php-esp32 location. |

## `monitor`

Open the serial console (`idf.py monitor`). Leave it with `Ctrl-]`. On a networked board the boot log
prints the address it came up on, which is where you point a browser for a `web-server` project.

| Flag | Meaning |
|---|---|
| `-p, --port <port>` | Serial port (default: `/dev/ttyACM*`, then autodetect). |
| `--idf-path <dir>` | ESP-IDF location. |
| `--php-esp32-path <dir>` | php-esp32 location. |

## `update-certs`

Refresh the TLS CA bundle a full-openssl `tls` project ships. `build` writes that bundle once and never
overwrites it; `update-certs` re-copies the host trust store (`[extensions.openssl] certs_source`, or
the auto-detected system store) over the old one, at `certs_path`. It reads the project config and
errors if the project does not build the TLS client. No flags.

```sh
phpflash update-certs
```

## `discover`

Identify whatever board is plugged in, from the board outwards, with no project needed. It lists the
serial ports and their USB bridge, probes the chip with esptool, and maps it to the supported boards,
printing a ready-to-paste `[board] target` line for the match.

```
$ phpflash discover
Chip:     ESP32-S3 (QFN56) (revision v0.2)
Target:   esp32s3
Flash:    16MB
Radio:    WiFi, Bluetooth (built into the chip)

Supported boards for esp32s3 (family "ESP32-S3"):
  - esp32-s3-eth    ESP32-S3-ETH     network: ethernet  microSD: yes (-DBOARD=esp32-s3-eth)

To target one from a project, set in php-esp32.config.toml:
  [board]
  target = "esp32-s3-eth"
```

Chip and built-in WiFi/BT come from the silicon. LAN and microSD are board-level, so `discover` reports
what each candidate board declares. A chip that matches no board is called out as unsupported.

`--all` settles a blank board where the chip alone cannot tell two models apart. It flashes a small
discovery firmware, built per candidate board so it uses that board's real wiring, brings up each one's
peripherals, and names the match. It overwrites the app on the board, so it asks first and reminds you
to re-flash.

| Flag | Meaning |
|---|---|
| `-p, --port <port>` | Serial port to probe (default: first detected). |
| `--no-probe` | List ports and USB only; do not reset the board. |
| `--all` | Actively probe peripherals by flashing a discovery firmware (erases the app, asks first). |
| `-y, --yes` | With `--all`, skip the confirmation prompt. |
| `--idf-path <dir>` | ESP-IDF location (for esptool and building the probe). |
| `--php-esp32-path <dir>` | php-esp32 location (boards list and discovery firmware). |
