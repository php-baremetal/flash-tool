---
eyebrow: 'Docs · Commands'
lede: 'Install the global prerequisites once per machine: ESP-IDF (the cross-toolchain) and the php-esp32 firmware sources, cloned at the versions php-esp32 targets and set up with their own installers. Idempotent — run it again later and it updates the checkouts instead of re-cloning.'
see_also:
  - { href: './init.md', meta: 'Commands', label: 'phpflash init' }
  - { href: './build.md', meta: 'Commands', label: 'phpflash build' }
  - { href: 'https://github.com/php-baremetal/php-esp32', meta: 'external', label: 'php-esp32 firmware' }
prev: { label: 'phpflash init', href: './init.md' }
next: { label: 'phpflash build', href: './build.md' }
---

# `phpflash system-setup`

`system-setup` installs the two things every build depends on: the ESP-IDF cross-toolchain and the
php-esp32 firmware sources. Run it once per machine. It clones each repository at the version php-esp32
targets, runs its installer, and leaves both under `~/esp` by default.

<!-- @code-block language="bash" label="terminal — system-setup" -->
```bash
phpflash system-setup
```
<!-- @endcode-block -->

## What it does

The command runs two steps in order, printing `==> ESP-IDF` and `==> php-esp32` as it goes. An error is
attributed to the step that failed.

<!-- @steps -->
- **ESP-IDF** — clone the repo (`--recursive`) at the resolved path, check it out at the requested git
  ref, then run its `install.sh esp32p4` (which brings the cross-compilers and a private Python
  environment, so there is nothing to install by hand).
- **php-esp32** — clone the firmware repo, check it out at the requested ref, then run its
  `scripts/fetch-php.sh` to download and patch the PHP source.
<!-- @endsteps -->

<!-- @callout variant="info" title="Idempotent" -->
If a target path already exists, `system-setup` checks it out to the requested version rather than
re-cloning. Run it again after a firmware update and it advances the existing checkouts. A fresh path
is cloned; an existing one is updated in place.
<!-- @endcallout -->

<!-- @callout variant="note" title="What fetch-php.sh pulls" -->
The PHP source is not committed to php-esp32 (it is large and reproducible from the official tarball).
`fetch-php.sh` downloads it, verifies its sha256, and applies the target patches. Optional extension
sources (oniguruma, SQLite, OpenSSL) are fetched on demand at build time, only when you enable the
extension that needs them.
<!-- @endcallout -->

## Where things land

By default everything goes under `~/esp`: ESP-IDF at `~/esp/esp-idf`, php-esp32 at `~/esp/php-esp32`.
Each location is resolved by precedence — the command flag wins, then the value in a project config if
one is present in the current directory, then the environment variable, then the default.

| Path | Flag | Config | Environment | Default |
|---|---|---|---|---|
| ESP-IDF | `--idf-path` | `[esp-idf] path` | `IDF_PATH` | `~/esp/esp-idf` |
| php-esp32 | `--php-esp32-path` | `[php-esp32] path` | `PHP_ESP32_DIR` | `~/esp/php-esp32` |

The git refs resolve the same way, minus the environment step: the flag, then the project config, then
a built-in default.

| Ref | Flag | Config | Default |
|---|---|---|---|
| ESP-IDF version | `--idf-version` | `[esp-idf] version` | `v5.5.5` |
| php-esp32 version | `--php-esp32-version` | `[php-esp32] version` | `master` |

## Flags

<!-- @params -->
<!-- @param name="--idf-path" type="string" -->
Where to install ESP-IDF. Defaults through `[esp-idf] path`, then `IDF_PATH`, then `~/esp/esp-idf`.
<!-- @endparam -->
<!-- @param name="--idf-version" type="string" -->
The ESP-IDF git ref to check out. Defaults through `[esp-idf] version`, then `v5.5.5` — the version
php-esp32 targets.
<!-- @endparam -->
<!-- @param name="--php-esp32-path" type="string" -->
Where to install php-esp32. Defaults through `[php-esp32] path`, then `PHP_ESP32_DIR`, then
`~/esp/php-esp32`.
<!-- @endparam -->
<!-- @param name="--php-esp32-version" type="string" -->
The php-esp32 git ref to check out. Defaults through `[php-esp32] version`, then `master`.
<!-- @endparam -->
<!-- @endparams -->

<!-- @code-block language="bash" label="terminal — relocate the installs" -->
```bash
phpflash system-setup --idf-path ~/toolchains/esp-idf --php-esp32-path ~/src/php-esp32
```
<!-- @endcode-block -->

## Doing it by hand

`system-setup` is a convenience over standard ESP-IDF and php-esp32 setup. You can perform the same
steps yourself: clone [ESP-IDF](https://github.com/espressif/esp-idf) at the target version and run its
`install.sh`, then clone [php-esp32](https://github.com/php-baremetal/php-esp32) and run
`scripts/fetch-php.sh`. Point [`phpflash build`](./build.md) at the results with `--idf-path` /
`--php-esp32-path`, the `[esp-idf]` / `[php-esp32]` config tables, or the `IDF_PATH` / `PHP_ESP32_DIR`
environment variables.

Once this has run, scaffold a project with [`phpflash init`](./init.md) and build it with
[`phpflash build`](./build.md).
