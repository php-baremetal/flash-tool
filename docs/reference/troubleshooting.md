---
eyebrow: 'Docs · Reference'
lede:    'The guardrails phpflash puts around a flash, and the failure modes you actually hit — a chip that does not match the project board, a stale embedded image shadowing the card, a cable or port that is not there, a missing toolchain, a mispinned target, and HTTPS that needs a refreshed CA bundle. Symptom, cause, fix for each.'
see_also:
  - { href: './design.md', meta: 'Reference', label: 'Design' }
  - { href: '../commands/flash.md', meta: 'Commands', label: 'phpflash flash' }
  - { href: '../commands/update-certs.md', meta: 'Commands', label: 'phpflash update-certs' }
prev: { label: 'Design', href: './design.md' }
next: { label: 'No next page', href: '#' }
---

# Troubleshooting

phpflash puts guardrails around the destructive steps and fails with an actionable message rather than
letting ESP-IDF or esptool error cryptically mid-operation. This page walks the failure modes you are
likely to hit, each as symptom → cause → fix.

## Chip does not match the project board

Before writing, `flash` probes the connected chip and refuses if its ESP-IDF target differs from the
project board's family target — an S3 image with a P4 plugged in, say. The message names both sides.

<!-- @code-block language="text" label="The board-mismatch error" -->
```text
board mismatch: this project targets "esp32-s3-eth" (esp32s3), but the connected chip is
ESP32-P4 (esp32p4).
  Fix [board].target in php-esp32.config.toml, plug in the right board, or re-run with --force.
```
<!-- @endcode-block -->

| Symptom | Cause | Fix |
|---|---|---|
| `flash` aborts with `board mismatch` before writing | The connected chip's target is not the target of the project board's family | Plug in the right board; or correct `[board].target` in `php-esp32.config.toml` and rebuild; or, if you know what you are doing, re-run `phpflash flash --force`. |

<!-- @callout variant="info" title="The check is deliberately lenient" -->
The check only aborts on a *confirmed* mismatch. If the board's family target cannot be resolved, or
the probe itself fails to run (no board yet, a busy port), the flash proceeds — esptool verifies the
chip again during the actual write, so it stays the backstop. `--force` skips the check entirely.
<!-- @endcallout -->

## A stale embedded image shadows the microSD

The firmware runs `/app/index.php` (the embedded FAT image) in preference to `/sdcard/index.php` (the
card). A `microsd` project ships no embedded image, so a leftover one from an earlier `embedded` build
— of this or another project — would still mount at `/app` and be run silently, with no error. To
prevent that, `flash` erases the `storage` partition after a `microsd` flash.

| Symptom | Cause | Fix |
|---|---|---|
| After flashing a `microsd` project, the board runs old code that is not on the card, with no error | An embedded FAT image left in the `storage` partition by an earlier `embedded` build mounts at `/app`, which the firmware prefers over the card | Nothing to do — a `microsd` flash erases `storage` automatically. If you ran an older flow, re-run `phpflash flash`; the erase makes the embedded mount fail so the firmware falls back to the microSD. |

<!-- @callout variant="note" title="The erase is best-effort" -->
A board with no `storage` partition (or one that cannot be reached) is not a flash failure: the erase
reports a harmless note and is swallowed rather than returned. It never fails a flash on its own.
<!-- @endcallout -->

## No board, wrong cable, or the port is not found

phpflash finds the serial port from `-p`, then the config's `[board].port`, then the first serial
device that exists — `/dev/ttyACM*` first (the P4-Pico's on-board CH343P bridge), then `/dev/ttyUSB*`
— else it leaves the port empty for ESP-IDF to autodetect. Detection only checks that the device node
exists; it never opens the port. `discover` fails outright when nothing is found.

<!-- @code-block language="text" label="discover with no serial device" -->
```text
no serial device found (looked for /dev/ttyACM*, /dev/ttyUSB*). Plug the board in and check the USB cable
```
<!-- @endcode-block -->

| Symptom | Cause | Fix |
|---|---|---|
| `no serial device found (looked for /dev/ttyACM*, /dev/ttyUSB*)` | No matching device node, most often a charge-only USB cable or the board not plugged in | Use a data-carrying USB cable; plug the board in; confirm a `/dev/ttyACM*` or `/dev/ttyUSB*` node appears. |
| `flash`/`monitor` autodetect grabs the wrong device | Another USB-serial device enumerated first and no port was set | Pass `-p /dev/ttyACM0` (or the right node), or set `[board].port` in the config. |
| `serial port /dev/ttyACM0 is not accessible (permission denied)` | The user is not in the `dialout` group that owns the serial device (a common first run on Linux) | `flash`/`monitor` check this up front and print the fix: `sudo chmod a+rw /dev/ttyACM0` for this port now, or `sudo usermod -aG dialout $USER` then log out/in (or `newgrp dialout`) permanently. |

## The toolchain or firmware is not installed

Every build/flash command loads the project config, then resolves php-esp32 (and ESP-IDF) from the
flag, config, env (`PHP_ESP32_DIR`, `IDF_PATH`) and default. A missing install fails with a pointer to
`system-setup`.

| Symptom | Cause | Fix |
|---|---|---|
| `no php-esp32.config.toml in this directory (run \`phpflash init\` first)` | The command ran outside a project directory | `cd` into the project, or scaffold one with `phpflash init`. |
| `php-esp32 not found at <path> (run \`phpflash system-setup\`)` | The firmware repo is not installed where phpflash looked | Run `phpflash system-setup`; or point at an existing checkout with `--php-esp32-path` / `[php-esp32] path` / `PHP_ESP32_DIR`. |
| The build fails invoking `idf.py` (export.sh not found, toolchain missing) | ESP-IDF is not installed, or `--idf-path` / `IDF_PATH` points at the wrong place | Run `phpflash system-setup` (installs ESP-IDF and runs its `install.sh`); or set the correct `--idf-path`. |
| `xtensa-esp32s3-elf-gcc ... is not a full path and was not found in the PATH` (or the RISC-V equivalent) | The toolchain for the board's architecture was never installed — an ESP-IDF `install.sh` run that omitted this target (the ESP32-S3 is Xtensa, the ESP32-P4 is RISC-V; they are different toolchains) | Install it: `~/esp/esp-idf/install.sh esp32s3` (or `esp32p4`), then `phpflash build --clean`. `phpflash system-setup` now installs both (`esp32s3,esp32p4`), so setups done through it are covered. |

<!-- @callout variant="tip" title="system-setup is idempotent" -->
Re-running `system-setup` is safe: an existing checkout is updated to the requested version rather
than re-cloned (and its ESP-IDF submodules re-pinned to that version), and an error is attributed to
the step that failed (ESP-IDF's `install.sh`, or php-esp32's `fetch-php.sh`).
<!-- @endcallout -->

## A build failure repeats after you fixed the environment

CMake caches the results of a failed *configure* — a missing compiler shows up as
`compiler identification is unknown` and is remembered — so the same error repeats even after you fix
the toolchain, submodules or IDF version. Wipe the build tree and reconfigure.

| Symptom | Cause | Fix |
|---|---|---|
| The identical configure error repeats after fixing the toolchain / submodules / IDF version | The failed configure is cached in the project's `build/` tree (`CMakeCache.txt` remembers "compiler unknown") | `phpflash build --clean` (removes the build directory first, then reconfigures). A build failure also prints this hint. |
| `Cannot specify link libraries for target "mbedcrypto" which is not built by this project` | The ESP-IDF submodules are off their pins — typically a stray `git submodule update --remote` left `mbedtls` at 4.x, where the `mbedcrypto` target was removed (IDF 5.5 expects mbedtls 3.6.x) | `cd $IDF_PATH && git submodule update --init --recursive`, then `phpflash build --clean`. Never update IDF submodules with `--remote`. `system-setup` re-pins them for you when it manages the checkout. |

## Wrong architecture / build target mismatch

phpflash pins the ESP-IDF target from the board's family (`-DIDF_TARGET=<target>`). Without it,
`idf.py` guesses the target from any stray in-source `sdkconfig`, which silently builds the wrong
architecture when that file is left over from a different family. The per-project build tree keeps that
from happening across projects.

| Symptom | Cause | Fix |
|---|---|---|
| A build produces an image for the wrong chip, or `flash` then reports `board mismatch` | A leftover in-source `sdkconfig` from another family was picked up | The pinned `-DIDF_TARGET` prevents this for phpflash builds; make sure you are building through `phpflash build` (isolated per-project tree) rather than a stray `idf.py` in the firmware tree. |
| `PHP version "<x>" is not installed in <path> (available: ...)` | `[php].version` pins a version the installed php-esp32 does not carry | Set `[php].version` to one of the listed available versions, leave it empty for the firmware default, or add the version to php-esp32. |

## HTTPS fails until the CA bundle is refreshed

A full-openssl `tls` project ships a CA bundle so the device can verify HTTPS peers. `build` writes
that bundle **once** and never overwrites it; `update-certs` re-copies the host trust store over the
old one to pick up new or renewed roots. With no bundle at all the client connects but does not verify
(and the firmware logs that).

| Symptom | Cause | Fix |
|---|---|---|
| HTTPS peer verification fails on the device with a valid certificate | The shipped CA bundle is stale (a renewed or newly added root CA is missing) | Run `phpflash update-certs` to re-copy the host root store, then rebuild and flash. |
| `update-certs` errors that the project does not build the TLS client | The project is not a full-openssl build with the `tls` setting, so there are no certificates to manage | Enable the full openssl build with `tls` in the config, or drop the expectation of on-device HTTPS verification. |
| `update-certs` errors that `certs_path` is an absolute on-device path | `[extensions.openssl] certs_path` points at a path phpflash cannot manage from the host | Use a project-relative `certs_path` (the default is `certs/ca-bundle.crt` under the source), or manage that file yourself. |

<!-- @callout variant="note" title="build ships the bundle once, on purpose" -->
`build` copies the host's root CAs into the project the first time and then leaves them alone, so a
bundle you curated is never silently replaced by a build. `update-certs` is the explicit way to
refresh it (a renewed root, a different `certs_source`, the auto-detected system store).
<!-- @endcallout -->
