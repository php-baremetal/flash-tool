# phpflash

`phpflash` is the command-line frontend for
[php-esp32](https://github.com/php-baremetal/php-esp32): it creates, configures, builds, flashes
and monitors a PHP-on-ESP32 project without touching ESP-IDF by hand. It's a single static Go
binary and it replaces php-esp32's `setup.sh` / `flash.sh` / `monitor.sh`.

It knows nothing hard-coded about extensions or boards: the list of extensions, their build
flags, the chip families and boards, and which storage/execution modes each supports all come
from the installed php-esp32 repository. Add a board or an extension to php-esp32 and it shows up
in `phpflash` automatically.

## Install

### From a release (Linux)

Download the binary for your architecture and verify its checksum (each release ships a
`SHA256SUMS`). The `latest/download` URL always points at the newest release:

```sh
BASE=https://github.com/php-baremetal/flash-tool/releases/latest/download
curl -LO "$BASE/phpflash-linux-amd64"      # or phpflash-linux-arm64
curl -LO "$BASE/SHA256SUMS"
sha256sum --ignore-missing -c SHA256SUMS
chmod +x phpflash-linux-amd64
sudo mv phpflash-linux-amd64 /usr/local/bin/phpflash
phpflash --version
```

To update, just re-run these steps. To pin a specific version, replace `latest/download` with
`download/<tag>` (e.g. `download/v1.0.0`).

### From source

With Go 1.22 or newer:

```sh
go build -o phpflash .
```

Put the resulting `phpflash` binary on your `PATH`.

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

### `phpflash init [dir]`

Scaffolds a new project (in `dir`, or the current directory). Interactive, with a sensible
default for every prompt; pass `--yes` to accept all defaults non-interactively. It refuses to
overwrite an existing config unless `--force`.

It creates:

```
my-project/
├── php-esp32.config.toml     # the project configuration
├── .gitignore
└── project-src/
    └── index.php             # your PHP entry point
```

`project-src/` is the **deployable**: on a `microsd` project it's what you copy to the card
(your `index.php`, its `vendor/`, any other files), kept apart from the config and the build
output.

The prompts, in order:

1. **Project name** (default: the directory name)
2. **Chip family**, then **board** — listed from the installed php-esp32
3. **Storage type** and **execution type** — only the modes the firmware implements *and* the
   chosen board supports are offered
4. **Optional extensions** and their settings — read from php-esp32's manifest; each project
   type's mandatory extensions are always on and not shown as toggles
5. **Serial port** (default: empty = autodetect at flash time)
6. **Starter** — `hello` or `blink`

If php-esp32 isn't installed yet, the board/extension steps are skipped with a note; you can run
`init` again after `system-setup`, or edit the config by hand.

Flags: `--name`, `--board`, `--php-esp32-path`, `--yes`, `--force`.

### `phpflash system-setup`

Installs the global prerequisites (replaces `setup.sh`):

1. **ESP-IDF** — clone/checkout at the resolved version, then run its `install.sh esp32p4`.
2. **php-esp32** — clone/checkout the firmware repo, then run its `scripts/fetch-php.sh`.

Idempotent: an existing checkout is updated to the requested version instead of re-cloned.
Errors are attributed to the step (ESP-IDF vs php-esp32) that failed.

Flags: `--idf-path`, `--idf-version`, `--php-esp32-path`, `--php-esp32-version`.

### `phpflash build`

Builds the firmware for the project. It reads the config, turns the enabled extensions into a
deterministic list of `-D<flag>=ON/OFF` arguments (from php-esp32's manifest), runs any fetch
scripts an enabled extension needs, then drives ESP-IDF:

```
idf.py -B ./build/compiled -DSDKCONFIG=./build/sdkconfig \
       -DBOARD=<board> -DPHP_VERSION=<version> -D<flag>… build
```

The full ESP-IDF build tree lives under the project's `build/compiled/`; on success the
flashable images (`php-esp32.bin`, `bootloader.bin`, `partition-table.bin`) are copied up into
`build/`. Because the build output and `sdkconfig` are per-project, several projects can share
one php-esp32 install with isolated, side-by-side builds.

For an `embedded` project (`storage_type = "embedded"`), the build also passes
`-DPHP_EMBED_SRC=<project-src>`, so php-esp32 packs your PHP source into a read-only image
(`storage.bin`) that `flash` writes into the chip — the board then runs without a card. A
`microsd` project embeds nothing; you copy `project-src/` to the card yourself.

The build is shown as phases with a compile progress bar. If it fails, the full ESP-IDF output
is printed so you can see the real error.

Flags: `--idf-path`, `--php-esp32-path`.

### `phpflash flash [-p port]`

Builds (if needed) and flashes the firmware to the board (`idf.py flash`). The serial port comes
from `-p`, else the config's `[board].port`, else the first serial device that exists —
`/dev/ttyACM*` (the Pico's on-board CH343P bridge) before `/dev/ttyUSB*` — else ESP-IDF's
autodetect. Detection only checks that the device node exists; it never opens a port, so it can't
disturb an unrelated device.

### `phpflash monitor [-p port]`

Opens the serial monitor (`idf.py monitor`).

## The project config: `php-esp32.config.toml`

Written by `init`, read by the other commands.

```toml
name = "my-project"
storage_type = "microsd"      # where the PHP source lives (microsd | embedded)
type = "init-loop"            # execution model (init-loop | web-server | event-driven)

[board]
target = "esp32-p4-pico"      # the board (its family decides the ESP-IDF target)
port   = ""                   # serial port; empty = autodetect at flash time

# Toolchain and firmware sources. An empty `path`/`version` means "use the default".
[esp-idf]
path    = ""                  # e.g. ~/esp/esp-idf
version = "v5.5.5"            # git ref

[php-esp32]
path    = ""
version = ""                  # git ref (branch/tag); empty = default branch

# One table per enabled extension, with `enabled` plus any settings it declares.
[extensions.sqlite]
enabled = true

[extensions.mbstring]
enabled = true
onig    = true                # a setting of the mbstring extension

[php]
src   = "project-src"         # the PHP source folder (copied to the SD / embedded)
entry = "index.php"           # entry file within src
```

### Path and version resolution

The ESP-IDF and php-esp32 locations (and versions) resolve by precedence, highest wins:

1. **CLI flag** (`--idf-path`, `--php-esp32-path`, `--idf-version`, `--php-esp32-version`)
2. **`php-esp32.config.toml`** `path` / `version`
3. **Environment** — `IDF_PATH`, `PHP_ESP32_DIR`
4. **Default** — `~/esp/esp-idf`, `~/esp/php-esp32`

## How it discovers php-esp32

Everything about extensions and hardware is owned by php-esp32 and read at runtime:

- the repo descriptor `php-esp32.toml` (default PHP version and board),
- the per-version extension manifest `components/php/versions/<version>/manifest.toml`,
- the board descriptors `boards/<family>/<board>/board.toml` and `boards/<family>/family.toml`.

See [docs/design.md](docs/design.md) for the full architecture.

## Development

```sh
go test ./...
go vet ./...
go build -o phpflash .
```

## Status

`init`, `system-setup`, `build`, `flash` and `monitor` are implemented, producing firmware by
driving a local ESP-IDF. Delivering prebuilt firmware images (to skip the local toolchain) is a
possible future addition — see [docs/design.md](docs/design.md).
