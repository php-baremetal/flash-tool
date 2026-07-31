# Design

phpflash is the user-facing frontend for the [php-esp32](https://github.com/php-baremetal/php-esp32)
firmware. The two live side by side under `php-baremetal/` as separate repositories: php-esp32 is
the firmware (an ESP-IDF project), phpflash is the tool a PHP developer runs.

## Goals

- Let a PHP developer create, configure, build, flash and monitor a project without learning
  ESP-IDF.
- Ship as a single, cross-platform binary (Linux and macOS now; Windows is a localized addition
  later).
- Hold **no hard-coded knowledge** of extensions or hardware. The list of extensions, their build
  flags and settings, the chip families and boards, and which storage/execution modes each
  supports all come from php-esp32. phpflash reads that, presents it, records the choices, and
  emits the right build flags — nothing more.

## Language and libraries

- **Go** — a single static binary, trivial cross-compilation, and a natural fit for a tool that
  orchestrates external processes (git, ESP-IDF's `install.sh`, `idf.py`) and scaffolds files.
- **[cobra](https://github.com/spf13/cobra)** for the command tree.
- **[BurntSushi/toml](https://github.com/BurntSushi/toml)** for decoding the config and the
  php-esp32 descriptors.
- **`go:embed`** for the scaffolding templates.

Only those two third-party libraries; everything else is the standard library, including the
interactive prompts (a small terminal prompter behind an interface, so command logic is tested
without a TTY).

## The contract with php-esp32

phpflash reads three kinds of file from an installed php-esp32:

- **`php-esp32.toml`** (repo root) — the default PHP version and the default board.
- **`components/php/versions/<version>/manifest.toml`** — the extension manifest, *per PHP
  version*. It lists every extension the firmware supports, and for each one: its build flag (if
  any — core extensions are always compiled and carry none), its settings (each with its own
  flag and default), any fetch script it needs, the extensions it `requires`, and the project
  types it is mandatory for (`required_for`). It also declares the storage and execution modes
  the firmware implements.
- **`boards/<family>/<board>/board.toml`** and **`boards/<family>/family.toml`** — the board and
  chip-family descriptors: a board's name and the storage/execution modes its hardware supports;
  a family's ESP-IDF target.

The build flags for a project are derived entirely from the manifest: the project type's
mandatory extensions, plus the enabled optional ones (with their `requires` pulled in
transitively) and their settings, become the `-D<flag>=ON` arguments; every other optional flag
the manifest knows is emitted as `=OFF`, so a build is deterministic regardless of what a shared
build directory happened to hold before. Extensions with no flag are simply always compiled.

Because all of this lives in php-esp32, adding an extension, a setting, a board or a per-type
rule there is picked up by phpflash with no code change.

## Storage type and project type

A project declares two orthogonal things, both drawn from the manifest:

- **`storage_type`** — where the PHP source lives. `microsd` (the source is on the card and the
  firmware reads it) is implemented; `embedded` (the source compiled into the firmware) is
  reserved.
- **`type`** — the execution model. `init-loop` (run `index.php`, then `setup()` once and
  `loop($tick)` repeatedly) is implemented; `web-server` and `event-driven` are reserved.

What the *firmware* implements and what a *board* can physically do are separate concerns: a mode
is offered only when it is both implemented (the manifest) and supported by the chosen board (its
`board.toml`). So `web-server` never appears for a board with no network, even though the
firmware side is identical. `init` offers the intersection; anything else is unavailable.

## Producing firmware

phpflash builds firmware by **driving a local ESP-IDF** — the toolchain `system-setup` installs.
`build` runs the needed fetch scripts and then `idf.py … build`, into a per-project build
directory:

```
idf.py -B <project>/build/compiled -DSDKCONFIG=<project>/build/sdkconfig \
       -DBOARD=<board> -DPHP_VERSION=<version> -D<flag>… build
```

The full ESP-IDF tree stays under `build/compiled/`, the `sdkconfig` next to it, and the
flashable `.bin` images are copied up into `build/`. Keeping the build per-project means several
projects share one php-esp32 install without clobbering each other, and the php-esp32 source tree
stays clean.

Two other firmware-delivery strategies were considered and left for later: downloading **prebuilt
images** (light, no local ESP-IDF, but needs a distribution channel and only covers published
extension combinations), and a **hybrid** of the two. The config is shaped so any of them can be
added without breaking existing projects.

## Package structure

```
phpflash/
├── main.go                 # entry point
├── cmd/                    # cobra commands: root, init, system-setup, build, flash, monitor
├── internal/
│   ├── config/             # the config schema, load/save, path/version resolution
│   ├── manifest/           # reading php-esp32's descriptors; the effective extension/flag set
│   ├── prompt/             # the Prompter interface, a terminal impl and a scripted fake
│   ├── project/            # init scaffolding
│   ├── setup/              # system-setup orchestration behind a Runner interface
│   ├── build/              # build/flash/monitor orchestration + the idf.py invoker + progress
│   ├── platform/           # the small OS shims (home dir, install-script name)
│   └── templates/          # the embedded scaffolding files
└── docs/
```

## Testing

The parts worth testing are pure logic and are kept behind interfaces so they run without a real
toolchain, board or network:

- **config** — round-trips the config (including the `[extensions.<name>]` tables and their
  settings) and checks the flag > config > env > default precedence.
- **manifest** — parses php-esp32's descriptors and resolves a project's effective set
  (mandatory-for-type + enabled optional + transitive `requires`) into the `-D<flag>` list,
  rejecting unmet requirements.
- **build** — checks the exact `idf.py` argument list and that fetches run before the build; the
  progress renderer is fed synthetic ninja output and checked independently.
- **init** and **system-setup** — the prompts, the manifest and the git/shell steps sit behind
  interfaces, so scaffolding runs against a fake manifest into a temp directory and the
  orchestration (order, idempotency, resolution) is unit-tested with fakes.

The actual `idf.py`, `git clone` and serial I/O are integration concerns, exercised on a real
machine with a board attached.

## Cross-platform

Linux and macOS are supported now. The OS-specific bits — the home directory, `install.sh` vs
`install.bat`, serial-port autodetect — sit behind small shims in `internal/platform`, so adding
Windows is a localized change.
