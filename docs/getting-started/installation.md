---
eyebrow: 'Docs · Getting started'
lede: 'Install phpflash — a single static Go binary — from a release or build it from source, verify it runs, then let system-setup bring in the ESP-IDF toolchain and the php-esp32 firmware sources.'
see_also:
  - { href: './first-project.md', meta: 'Getting started', label: 'Your first project' }
  - { href: '../commands/system-setup.md', meta: 'Commands', label: 'phpflash system-setup' }
  - { href: 'https://github.com/php-baremetal/php-esp32', meta: 'external', label: 'php-esp32' }
prev: { label: 'Overview', href: '../overview.md' }
next: { label: 'Your first project', href: './first-project.md' }
---

# Installation

`phpflash` is a single static Go binary. It scaffolds a project, drives the ESP-IDF build, flashes the
board, and opens the serial console — replacing php-esp32's `setup.sh`, `flash.sh` and `monitor.sh`
with one consistent tool. Installing it is a matter of putting one binary on your `PATH`.

Nothing else is installed at this point. The cross-toolchain (ESP-IDF) and the
[php-esp32](https://github.com/php-baremetal/php-esp32) firmware sources come later, in one step, via
[`phpflash system-setup`](../commands/system-setup.md).

## Host requirements

- A POSIX host: **Linux or macOS**. Prebuilt release binaries are published for Linux; on macOS,
  build from source.
- **Go 1.25+** — only to build from source. A prebuilt binary needs nothing.
- A supported board over USB, and a USB cable that carries data. The boards themselves are documented
  in [php-esp32](https://github.com/php-baremetal/php-esp32).

## From a release (Linux)

Each release ships per-architecture binaries and a `SHA256SUMS` checksum file. The `latest/download`
URL always points at the newest release, so this fetches the current binary, verifies its checksum,
and installs it:

<!-- @code-block language="bash" label="terminal — install from a release" -->
```bash
BASE=https://github.com/php-baremetal/flash-tool/releases/latest/download
curl -LO "$BASE/phpflash-linux-amd64"      # or phpflash-linux-arm64
curl -LO "$BASE/SHA256SUMS"
sha256sum --ignore-missing -c SHA256SUMS
chmod +x phpflash-linux-amd64
sudo mv phpflash-linux-amd64 /usr/local/bin/phpflash
phpflash --version
```
<!-- @endcode-block -->

Two Linux architectures are published: `phpflash-linux-amd64` and `phpflash-linux-arm64`. Pick the one
that matches your host.

<!-- @callout variant="tip" title="Pinning a specific version" -->
To install a fixed release instead of the latest, replace `latest/download` in the `BASE` URL with
`download/<tag>` — for example `download/v1.0.0`. The rest of the commands are unchanged.
<!-- @endcallout -->

## From source

With Go 1.25+ installed, clone the flash-tool repository and build the binary:

<!-- @code-block language="bash" label="terminal — build from source" -->
```bash
go build -o phpflash .
```
<!-- @endcode-block -->

Then put the resulting `phpflash` somewhere on your `PATH` (for example `/usr/local/bin`). This is the
route to use on macOS, where no prebuilt binary is published.

## Verify the install

Confirm the binary is on your `PATH` and runs:

<!-- @code-block language="bash" label="terminal — verify" -->
```bash
phpflash --version
```
<!-- @endcode-block -->

Every command also accepts `--help`, so `phpflash <command> --help` prints that command's full flag
list at the terminal.

## Next: install the toolchain and firmware sources

Installing the binary does not install ESP-IDF or the firmware. That happens once per machine, with a
single command:

<!-- @code-block language="bash" label="terminal — system-setup" -->
```bash
phpflash system-setup
```
<!-- @endcode-block -->

This clones ESP-IDF and runs its installer (which brings the cross-compilers and a private Python
environment), then clones php-esp32 and runs its `scripts/fetch-php.sh` to download and patch the PHP
source. It is idempotent — run it again later and it updates the checkouts rather than re-cloning. By
default both land under `~/esp`; override with `--idf-path` and `--php-esp32-path`.

<!-- @callout variant="note" title="What system-setup pulls in" -->
The full behaviour, flags, and default locations are covered on the
[`system-setup`](../commands/system-setup.md) command page. Once it finishes, you are ready to
scaffold and build a project — see [Your first project](./first-project.md).
<!-- @endcallout -->
