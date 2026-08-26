---
eyebrow: 'Docs · Commands'
lede: 'Build the firmware if needed and write it to the board. Two guardrails bracket the write: a chip-versus-board check before it, and a storage-partition wipe after a microSD flash so a leftover embedded image cannot shadow the card.'
see_also:
  - { href: './build.md', meta: 'Commands', label: 'phpflash build' }
  - { href: './monitor.md', meta: 'Commands', label: 'phpflash monitor' }
  - { href: './discover.md', meta: 'Commands', label: 'phpflash discover' }
  - { href: '../../getting-started/quick-start.md', meta: '10 min', label: 'Quick start' }
prev: { label: 'phpflash build', href: './build.md' }
next: { label: 'phpflash monitor', href: './monitor.md' }
---

# `phpflash flash`

`phpflash flash` writes the built firmware image to the connected board. It reads the project config,
resolves a serial port, checks the connected chip matches the project's board, drives `idf.py flash`
(which builds first if the tree is stale), and — for a `microsd` project — erases the board's storage
partition afterward. Run it from the project directory.

<!-- @code-block language="bash" label="terminal — flash" -->
```bash
phpflash flash            # autodetect the port
phpflash flash -p /dev/ttyACM0
phpflash flash --force    # skip the chip-vs-board check
```
<!-- @endcode-block -->

## Flags

| Flag | Meaning |
|---|---|
| `-p, --port <port>` | Serial port. Empty tries `/dev/ttyACM*`, then `/dev/ttyUSB*`, then lets ESP-IDF autodetect. |
| `--force` | Flash even if the connected chip does not match the project's board (skips the board check). |
| `--idf-path <dir>` | ESP-IDF location (overrides config, env and default). |
| `--php-esp32-path <dir>` | php-esp32 location. |

## Which port it uses

The port is resolved in a fixed order, and the first source that yields a value wins:

<!-- @steps -->
- **`-p` / `--port`** — an explicit flag always wins.
- **`[board].port`** — the project config's serial port, if set.
- **First serial device that exists** — the lowest-numbered `/dev/ttyACM*` (the pattern the P4-Pico's
  CH343P bridge enumerates as), then `/dev/ttyUSB*`.
- **Autodetect** — if none of the above matched, the port is left empty and ESP-IDF autodetects.
<!-- @endsteps -->

Detection only checks that the device node exists; it never opens a port, so it cannot hang on or
disturb a device that is not the board.

## The pre-flash board check

Before writing, and unless `--force` is passed, `flash` probes the connected chip and compares it to
the project board's ESP-IDF target. This stops you from flashing an image built for one chip onto a
different one — an `esp32s3` image onto a connected P4, say — which `esptool` would otherwise fail on
cryptically, mid-write.

The probe runs `esptool.py --chip auto flash_id` against the port and reads the chip's ESP-IDF target
(`ESP32-P4` becomes `esp32p4`). The check is deliberately lenient, and only a **confirmed** mismatch
aborts:

<!-- @steps -->
- **Board target unresolved.** If the project's board is not tied to a family target, there is nothing
  to compare against, and the flash proceeds.
- **Probe could not run.** A board that is not there yet, a busy port, or any probe error does not
  block the flash — `esptool` verifies the chip again during the actual write, so it stays the backstop.
- **Targets match.** The flash proceeds, after printing `board check: <chip> (<target>) matches the
  project's <board>`.
- **Targets differ.** The flash is refused with a message naming both the project's target and the
  connected chip, and pointing at `[board].target`, plugging in the right board, or `--force`.
<!-- @endsteps -->

<!-- @code-block language="text" label="terminal — a confirmed mismatch aborts" -->
```
board mismatch: this project targets "esp32s3" (esp32-s3-eth), but the connected chip is ESP32-P4 (esp32p4).
  Fix [board].target in php-esp32.config.toml, plug in the right board, or re-run with --force.
```
<!-- @endcode-block -->

<!-- @callout variant="warning" title="Two guardrails bracket the write" -->
`flash` checks the connected chip against the project's board **before** writing and refuses on a
confirmed mismatch — pass `--force` to override, for instance when two boards in the same family differ
in a way the probe cannot see. After a `microsd` flash it also wipes the board's `storage` partition,
so a leftover embedded image cannot mount and shadow the card. Both run for you; neither needs a flag
to switch on.
<!-- @endcallout -->

## The write, and the storage cleanup

With the check passed, `flash` runs `idf.py flash` into the project's own build tree. `idf.py` builds
first if the tree is stale, so a `flash` on an unbuilt or changed project compiles before it writes —
there is no separate build step you must remember.

After the write, and only for a `microsd` project (any `storage_type` that is not `embedded`), `flash`
erases the board's `storage` partition:

<!-- @code-block language="bash" label="terminal — the storage wipe after a microsd flash" -->
```bash
==> idf.py flash
==> erasing 'storage' partition (microsd project: stops a leftover embedded image from shadowing the card)
```
<!-- @endcode-block -->

The `storage` partition is the read-only FAT-image slot an `embedded` build flashes the PHP source
into. A `microsd` project never writes it, so an image left there by an earlier `embedded` build — of
this or another project — survives a `microsd` flash and still mounts at `/app`. Because the firmware
prefers `/app` over `/sdcard`, the board would then silently run that stale source instead of the card,
with no error. Erasing the partition guarantees the embedded mount fails and the firmware falls back to
the microSD.

The erase is best-effort: it runs `parttool erase_partition --partition-name storage`, and a board with
no `storage` partition (or one that cannot be reached) is not a flash failure — the error is reported
and swallowed rather than returned.

<!-- @callout variant="note" title="Embedded projects skip the cleanup" -->
The storage wipe runs only when the project's `storage_type` is not `embedded`. An `embedded` project
*wants* its image in the `storage` partition, so the cleanup does not apply and its source is written
there by the flash itself.
<!-- @endcallout -->

## After flashing

Open the serial console with [`phpflash monitor`](./monitor.md) to watch the boot log. On a networked
board the log prints the address it came up on, which is where a `web-server` project is reachable.
