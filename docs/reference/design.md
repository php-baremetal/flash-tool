---
eyebrow: 'Docs · Reference'
lede:    'How phpflash is built and how it reads php-esp32: a single static Go binary that holds no hardware or extension knowledge, derives every build flag from the firmware manifest, drives ESP-IDF into an isolated per-project tree with a pinned target, and identifies an unknown board from the chip outwards.'
see_also:
  - { href: './troubleshooting.md', meta: 'Reference', label: 'Troubleshooting' }
  - { href: '../configuration/manifest.md', meta: 'Configuration', label: 'The php-esp32 manifest' }
  - { href: 'https://github.com/php-baremetal/php-esp32', meta: 'external', label: 'php-esp32 on GitHub' }
prev: { label: 'Customize the partition table', href: '../recipes/custom-partition-table.md' }
next: { label: 'Troubleshooting', href: './troubleshooting.md' }
---

# Design

`phpflash` is the user-facing front end for the
[php-esp32](https://github.com/php-baremetal/php-esp32) firmware. The two are separate repositories:
php-esp32 is the firmware (an ESP-IDF project), phpflash is the tool a PHP developer runs.

## Goals

- Let a PHP developer create, configure, build, flash and monitor a project, and identify an unknown
  board, without learning ESP-IDF.
- Ship as a single, cross-platform binary (Linux and macOS now; Windows is a localized addition
  later).
- Hold no hard-coded knowledge of extensions or hardware. The extensions, their build flags and
  settings, the chip families and boards, the PHP versions, and which storage and execution modes each
  supports all come from php-esp32. phpflash reads that, presents it, records the choices, and emits
  the right build flags, nothing more. Today that means two chip families (ESP32-P4 and ESP32-S3) and
  PHP 8.3, 8.4 and 8.5; the tool gains whatever php-esp32 adds without a change here.

## Language and libraries

Go is the implementation language: a single static binary, trivial cross-compilation, and a natural
fit for a tool that orchestrates external processes (git, ESP-IDF's `install.sh`, `idf.py`, esptool)
and scaffolds files.

| Dependency | Role |
|---|---|
| **Go 1.25+** | The language and toolchain; produces one static binary per platform. |
| **[cobra](https://github.com/spf13/cobra)** | The command tree. |
| **[BurntSushi/toml](https://github.com/BurntSushi/toml)** | Decoding the config and the php-esp32 descriptors. |
| **`go:embed`** | Bundling the scaffolding templates into the binary. |

Only those two third-party libraries; everything else is the standard library, including the
interactive prompts — a small terminal prompter behind an interface, so command logic is tested
without a TTY.

## The contract with php-esp32

phpflash reads three kinds of file from an installed php-esp32:

- **`php-esp32.toml`** (repo root): the default PHP version and the default board.
- **`components/php/versions/<version>/manifest.toml`**: the extension manifest, per PHP version. It
  lists every extension the firmware supports, and for each: its build flag (core extensions carry
  none), its settings (each with its own flag and default), any fetch script it needs, the extensions
  it `requires`, and the project types it is mandatory for (`required_for`). It also declares the
  storage and execution modes the firmware implements.
- **`boards/<family>/<board>/board.toml`** and **`boards/<family>/family.toml`**: the board and
  chip-family descriptors. A board's name, and the storage and execution modes and peripherals
  (network interface, microSD) its hardware provides; a family's ESP-IDF target.

The build flags for a project are derived entirely from the manifest: the project type's mandatory
extensions, plus the enabled optional ones (with their `requires` pulled in transitively) and their
settings, become the `-D<flag>=ON` arguments; every other optional flag the manifest knows is emitted
as `=OFF`, so a build is deterministic regardless of what a shared build directory held before.
Extensions with no flag are always compiled.

<!-- @callout variant="info" title="Adding to php-esp32 needs no change here" -->
Because all of this lives in php-esp32, adding an extension, a setting, a board, a PHP version or a
per-type rule there is picked up by phpflash with no code change, and
`php-esp32/scripts/check-manifest.py` keeps the manifest in step with the actual CMake build.
<!-- @endcallout -->

## Storage type and project type

A project declares two orthogonal things, both drawn from php-esp32:

- **`storage_type`**: where the PHP source lives. `microsd` (read from the card) and `embedded`
  (compiled into a read-only image flashed into the chip) are both implemented.
- **`type`**: the execution model. `init-loop` (run `index.php`, then `setup()` once and `loop($tick)`
  repeatedly) and `web-server` (an HTTP server runs PHP per request) are implemented; `event-driven`
  is reserved.

What the firmware implements and what a board can physically do are separate concerns: a mode is
offered only when it is both implemented (the manifest) and supported by the chosen board
(`board.toml`). So `web-server` never appears for a board with no network. `init` offers the
intersection.

## Producing firmware

phpflash builds firmware by driving a local ESP-IDF, the toolchain `system-setup` installs. `build`
runs the needed fetch scripts and then `idf.py ... build` into a per-project build directory:

<!-- @code-block language="bash" label="The idf.py invocation" -->
```bash
idf.py -B <project>/build/compiled -DSDKCONFIG=<project>/build/sdkconfig \
       -DBOARD=<board> -DPHP_VERSION=<version> -DIDF_TARGET=<target> -D<flag>... build
```
<!-- @endcode-block -->

The full ESP-IDF tree stays under `build/compiled/`, the `sdkconfig` next to it, and the flashable
`.bin` images are copied up into `build/`. Keeping the build per-project means several projects share
one php-esp32 install without clobbering each other, and the php-esp32 source tree stays clean. The
`-DIDF_TARGET` is resolved from the board's family, so ESP-IDF never guesses the target from a stray
in-source `sdkconfig` and builds the wrong architecture.

Beyond the base flags, the project's config adds more as needed:

| Flag | When it is passed |
|---|---|
| `-DPHP_EMBED_SRC=<project-src>` | An `embedded` project — php-esp32 packs the source into a read-only image. |
| `-DPHP_NET_DNS=<servers>` | A networked project sets static DNS (`[network] dns`). |
| `-DPHP_TLS_CAFILE=<path>` | A full-openssl `tls` project — the CA bundle location. |
| `-DPHP_OPENSSL_CONF=<path>` | A full-openssl project with a custom `openssl.cnf` path. |

<!-- @callout variant="warning" title="Two guardrails sit around flash" -->
Before writing, phpflash probes the connected chip and refuses if its target does not match the
project board's family, so an image built for one chip is not pushed onto another (overridable with
`--force`, and skipped silently if the probe cannot run, since esptool verifies the chip during the
write anyway). After a `microsd` flash, it erases the `storage` partition, because a `microsd` project
ships no embedded image and the firmware prefers an embedded image over the card, so a leftover one
from an earlier build would otherwise shadow it.
<!-- @endcallout -->

Two other firmware-delivery strategies were considered and left for later: downloading prebuilt images
(no local ESP-IDF, but it needs a distribution channel and only covers published extension
combinations), and a hybrid of the two. The config is shaped so either can be added without breaking
existing projects.

## Board discovery

`discover` identifies a connected board from the board outwards, with no project:

<!-- @steps -->
1. **Ports and USB.** Glob `/dev/ttyACM*` and `/dev/ttyUSB*` (never opening a device) and read the USB
   descriptors from sysfs.
2. **Chip.** Run esptool's `flash_id` to read the chip type and target, revision, flash size, MAC and
   features; the built-in radio (WiFi/BT) is derived from the features. The target is normalized to
   what a family declares (the package suffix esptool prints, like `(QFN56)`, is dropped).
3. **Boards.** Map the chip's target to a family and list its boards, each annotated with the network
   and microSD it declares, plus a ready-to-paste `[board] target` snippet; an unmatched target is
   reported as an unsupported board.
<!-- @endsteps -->

`discover --all` goes further for a blank board, where the chip alone cannot distinguish models that
share the silicon. It flashes a small discovery firmware (`php-esp32/tools/discover-fw/`) built per
candidate board, reusing that board's `board.c`, so the probe uses each board's real GPIO wiring
rather than a hardcoded map. It brings up each candidate's peripherals (Ethernet, microSD), reads the
result over serial, and names the board whose peripherals come up. It is destructive (it overwrites the
app), so it confirms first and reminds the user to re-flash.

## Package structure

<!-- @code-block language="text" label="Repository layout" -->
```text
phpflash/
├── main.go                 # entry point
├── cmd/                    # cobra commands: root, init, system-setup, build,
│                           #   flash, monitor, update-certs, discover, ext
├── internal/
│   ├── config/             # config schema, load/save, path/version resolution
│   ├── manifest/           # reading php-esp32's descriptors; the effective flag set; board info
│   ├── prompt/             # the Prompter interface, a terminal impl and a scripted fake
│   ├── project/            # init scaffolding
│   ├── setup/              # system-setup orchestration behind a Runner interface
│   ├── build/              # build/flash/monitor + the idf.py invoker + progress + cert provisioning
│   ├── discover/           # chip probe, board matching, and the discovery-firmware flow
│   ├── platform/           # OS shims (home dir, serial-port listing, USB descriptors)
│   └── templates/          # the embedded scaffolding files
└── docs/
```
<!-- @endcode-block -->

## Testing

The parts worth testing are pure logic, kept behind interfaces so they run without a real toolchain,
board or network:

| Package | What its tests cover |
|---|---|
| **config** | Round-trips the config (including `[extensions.<name>]` tables, string settings and `[network]`) and checks the flag, config, env, default precedence. |
| **manifest** | Parses php-esp32's descriptors and resolves a project's effective set into the `-D<flag>` list, rejecting unmet requirements; exposes each board's peripherals and its family target. |
| **build** | Checks the exact `idf.py` argument list, that fetches run first, the embed, DNS and TLS argument derivation, the storage-partition erase, and the CA-bundle provisioning; the progress renderer is checked on synthetic ninja output. |
| **discover** | Parses esptool output and the discovery-firmware report, derives the chip target (dropping the package suffix) and built-in radio, and matches a chip to a family. |
| **init** and **system-setup** | The prompts, manifest and git/shell steps sit behind interfaces, so scaffolding runs against a fake manifest into a temp directory and the orchestration is unit-tested with fakes. |

The actual `idf.py`, `git`, esptool and serial I/O are integration concerns, exercised on a real
machine with a board attached.

<!-- @code-block language="bash" label="Development commands" -->
```bash
go test ./...
go vet ./...
go build -o phpflash .
```
<!-- @endcode-block -->

## Cross-platform

Linux and macOS are supported now. The OS-specific bits (the home directory, `install.sh` versus
`install.bat`, serial-port listing and USB-descriptor reading) sit behind small shims in
`internal/platform`, so adding Windows is a localized change.
