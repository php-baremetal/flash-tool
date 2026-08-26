---
eyebrow: 'Docs · Overview'
lede:    'phpflash is the command-line front end for php-esp32: one static Go binary that scaffolds, configures, builds, flashes and monitors a PHP-on-ESP32 project — holding no hard-coded hardware or extension knowledge, reading all of it from the installed firmware at runtime.'
see_also:
  - { href: './getting-started/installation.md', meta: 'Getting started', label: 'Installation' }
  - { href: './getting-started/first-project.md', meta: 'Getting started', label: 'Your first project' }
  - { href: 'https://github.com/php-baremetal/php-esp32', meta: 'external', label: 'php-baremetal/php-esp32 on GitHub' }
prev: { label: 'No previous page', href: '#' }
next: { label: 'Installation', href: './getting-started/installation.md' }
---

# Overview

`phpflash` is the command-line front end for [php-esp32](https://github.com/php-baremetal/php-esp32):
create, configure, build, flash and monitor a PHP-on-ESP32 project without touching ESP-IDF by hand.
It is a **single static Go binary**. It scaffolds a project, drives the ESP-IDF build, flashes the
board, opens the serial console, and can identify an unknown board that is plugged in. It replaces
php-esp32's `setup.sh`, `flash.sh` and `monitor.sh` with one consistent tool.

The two are separate repositories: php-esp32 is the firmware (an ESP-IDF project); phpflash is the
tool a PHP developer runs. This page is the mental model and the workflow at a glance; the pages that
follow install the tool and scaffold a first project.

## The shape of a project

A project is a folder with two parts: a `php-esp32.config.toml` that records the choices, and a
`project-src/` that holds the PHP you deploy. You write the code; `phpflash` reads the config and
does the rest. `project-src/` is the deployable, kept apart from the config and the build output.

<!-- @code-block language="toml" label="php-esp32.config.toml — a short one" -->
```toml
name         = "my-project"
storage_type = "microsd"      # microsd | embedded
type         = "init-loop"    # init-loop | web-server | event-driven

[board]
target = "esp32-p4-pico"      # its family decides the ESP-IDF target
port   = ""                   # empty = autodetect at flash time

[php]
src     = "project-src"
entry   = "index.php"
version = ""                  # empty = the firmware's default PHP version
```
<!-- @endcode-block -->

A sibling `php-esp32.config.local.toml`, if present, is overlaid on top for machine-specific tweaks
(a serial port, a toolchain path) and is git-ignored.

## The whole workflow at a glance

Five commands cover the lifecycle: `system-setup` once per machine, then `init`, `build`, `flash` and
`monitor` per project.

<!-- @code-block language="bash" label="From nothing to a running board" -->
```bash
phpflash system-setup            # once: install ESP-IDF and php-esp32
phpflash init my-project         # scaffold a project (config + project-src/)
cd my-project
$EDITOR project-src/index.php    # write your PHP
phpflash build                   # build the firmware
phpflash flash                   # check the board, then flash it
phpflash monitor                 # open the serial console
```
<!-- @endcode-block -->

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

## Everything comes from php-esp32 at runtime

The key idea: **phpflash holds no hard-coded knowledge of extensions or hardware.** The extensions
and their build flags and settings, the chip families and boards, the PHP versions, and which storage
and execution modes each supports are all read from the installed php-esp32 at runtime. phpflash reads
that, presents it, records the choices, and emits the right build flags — nothing more.

<!-- @callout variant="info" title="Add it to the firmware, and it appears here" -->
Add a board, an extension or a PHP version to php-esp32 and it appears in phpflash with no change to
the tool. `init` only ever offers what the firmware implements and the chosen board supports — so
`web-server` never appears for a board with no network, and an extension that a build does not carry
is never offered. Two chip families are supported today (ESP32-P4 and ESP32-S3) and PHP 8.3, 8.4 and
8.5; as php-esp32 grows, so does what phpflash offers.
<!-- @endcallout -->

phpflash reads three kinds of file from an installed php-esp32:

| File | What it declares |
|---|---|
| `php-esp32.toml` (repo root) | The default PHP version and the default board. |
| `components/php/versions/<version>/manifest.toml` | The per-version extension manifest: each extension's flag, settings, `requires`, `required_for`, fetch scripts, and the storage and execution modes the firmware implements. |
| `boards/<family>/<board>/board.toml` and `boards/<family>/family.toml` | The board and chip-family descriptors: a board's storage and execution modes and peripherals (network, microSD); a family's ESP-IDF target. |

A project's build flags are derived entirely from the manifest: the project type's mandatory
extensions, plus the enabled optional ones (with `requires` pulled in transitively) and their
settings, become `-D<flag>=ON`; every other optional flag the manifest knows is emitted `=OFF`, so a
build is deterministic regardless of what a shared build directory held before.

## Where to go next

- **[Installation](./getting-started/installation.md)** — get the binary, then let `system-setup`
  bring in the ESP-IDF toolchain and the php-esp32 firmware sources.
- **[Your first project](./getting-started/first-project.md)** — scaffold, build, flash and monitor a
  board end to end.
- **[Design](./reference/design.md)** — how phpflash is built and how it reads php-esp32.
- **[Troubleshooting](./reference/troubleshooting.md)** — the guardrails and the common failure modes.
