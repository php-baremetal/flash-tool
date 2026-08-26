---
eyebrow: 'Docs · Commands'
lede: 'Identify whatever board is plugged in, from the board outwards, with no project needed. Plain discover reads the chip and lists the supported boards that match it; --all actively probes a blank board''s peripherals — Ethernet, microSD — to name the exact model, and prints a boxed verdict you can build from.'
see_also:
  - { href: './flash.md', meta: 'Commands', label: 'phpflash flash' }
  - { href: './monitor.md', meta: 'Commands', label: 'phpflash monitor' }
  - { href: '../../getting-started/quick-start.md', meta: '10 min', label: 'Quick start' }
prev: { label: 'phpflash monitor', href: './monitor.md' }
next: { label: 'phpflash ext', href: './ext.md' }
---

# `phpflash discover`

`phpflash discover` identifies the board on the other end of the USB cable without any project or
config. It lists the serial ports and their USB bridge, probes the chip with `esptool`, and maps the
chip to the boards this php-esp32 install supports — printing a ready-to-paste `[board] target` line
for the match, or telling you the chip is not a supported board yet.

<!-- @code-block language="bash" label="terminal — discover" -->
```bash
phpflash discover          # identify the chip and list the boards that match it
phpflash discover --all    # actively probe a blank board's peripherals to name it
```
<!-- @endcode-block -->

## Flags

| Flag | Meaning |
|---|---|
| `-p, --port <port>` | Serial port to probe (default: the first detected device). |
| `--no-probe` | List ports and USB only; do not reset the board or probe the chip. |
| `--all` | Actively probe peripherals by flashing a discovery firmware. Erases the app; asks first. |
| `-y, --yes` | With `--all`, skip the confirmation prompt. |
| `--idf-path <dir>` | ESP-IDF location (for `esptool` and building the probe firmware). |
| `--php-esp32-path <dir>` | php-esp32 location (the boards list and the discovery firmware). |

## Passive identification (the default)

By default `discover` learns everything it can *without writing to the board*. It reads the serial
ports from sysfs, then talks to the chip's ROM loader with `esptool.py --chip auto flash_id`, which
briefly resets the board into download mode and hard-resets it after. From that it reports:

<!-- @code-block language="text" label="terminal — discover" -->
```
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
Note: `network`/`microSD` above are what each board *model* carries (from its
board.toml) -- the chip alone can't tell the models apart (same silicon).

To target one from a project, set in php-esp32.config.toml:
  [board]
  target = "esp32-s3-eth"
```
<!-- @endcode-block -->

The **Target** is the ESP-IDF target derived from the chip name (`ESP32-S3` becomes `esp32s3`, with any
package suffix like `(QFN56)` dropped), and it is the key boards are matched on. A chip that matches no
board here is called out as an unsupported board — the chip is fine, this install just has no board
definition for it — with a pointer to add one under `boards/<family>/<target>/`.

`--no-probe` stops before the chip probe: it lists the ports and their USB bridge only, never resetting
the board, and then lists the chip families this install knows about.

## The capability model: what the chip knows versus what a board carries

The split that runs through `discover` is which facts come from the **silicon** and which come from the
**board model**:

| Capability | Where it is known | How `discover` reports it |
|---|---|---|
| Chip, revision, flash, MAC | the silicon (esptool) | read directly and printed |
| **Radio** — built-in WiFi / Bluetooth / 802.15.4 | the silicon (esptool `Features:`) | read directly; the one network interface a chip probe genuinely reveals |
| **Network** — wired Ethernet | the board model (`board.toml`) | listed per candidate board; the chip cannot be probed for it |
| **microSD** — a card slot | the board model (`board.toml`) | listed per candidate board; the chip cannot be probed for it |

The consequence: several board models share one chip, so the chip probe alone often cannot tell them
apart — a P4-ETH and a P4-Pico are the same silicon. Plain `discover` therefore lists *every* supported
board in the family and leaves the choice to you. To have the tool decide, you have to actively exercise
the peripherals, which is what `--all` does.

## `--all`: actively probe the peripherals

`--all` settles a blank board the chip alone cannot place. It flashes a small **discovery firmware**,
built once **per candidate board** — reusing that board's `board.c`, so the probe uses each board's real
GPIO wiring rather than any hardcoded pins — brings up each board's peripherals, reads the result back
over serial, and names the match. Because it uses each board's own wiring, a newly added board's probe
follows automatically with no change here.

It works from the network outward: network-capable boards are probed first, because a link coming up is
the clearest discriminator and lets the run stop early. For each candidate it reports the wiring it used
and what came up:

<!-- @code-block language="text" label="terminal — discover --all" -->
```
WARNING: --all flashes a small discovery firmware to actively probe this
board (Ethernet, microSD), building it once per candidate board. This ERASES
the app on the board; you'll need to re-flash your firmware afterwards
(phpflash flash).
Continue? [y/N]: y

== Probing with the esp32-p4-eth wiring ==
  board wiring: ESP32-P4-ETH
  Ethernet:     up (10.42.0.224)
  microSD:      card present (14895MB)
  PSRAM:        32MB

====================================================
  THIS BOARD IS:  ESP32-P4-ETH
  build with:     -DBOARD=esp32-p4-eth
====================================================

The discovery firmware is on the board now -- re-flash your project:
  phpflash flash        (or: phpflash build && phpflash flash)
```
<!-- @endcode-block -->

The boxed verdict is the answer: `THIS BOARD IS: <name>` and the `-DBOARD=<key>` to build with. If the
probe cannot uniquely place the board — a network board whose cable is not plugged in, or an ambiguity
between a slotless `-zero` variant and a card board with an empty slot — it says so and suggests what to
change (connect the cable, or insert a card) rather than guessing.

<!-- @callout variant="warning" title="--all overwrites the app on the board" -->
`discover --all` flashes a probe firmware over whatever is on the board, so it warns, asks first, and
reminds you to re-flash your project afterward (`phpflash flash`). Pass `-y` / `--yes` to skip the
confirmation prompt in a script. Ethernet detection needs the cable connected, since the probe brings
the link up.
<!-- @endcallout -->

<!-- @callout variant="note" title="How a card settles the -zero ambiguity" -->
A mounted card proves the board physically has a slot, which rules out the slotless `-zero` variant. So
when no network link comes up but a card mounts, `--all` can still pick the single non-network board
that has a slot. With neither a link nor a card, a `-zero` board and a card board with an empty slot
look identical — the run only decides when there is a single non-network candidate overall.
<!-- @endcallout -->

## After discovering

Copy the `[board] target` line into `php-esp32.config.toml` (or build with `-DBOARD=<key>` by hand),
then [`build`](./build.md) and [`flash`](./flash.md). If you ran `--all`, the discovery firmware is
still on the board, so re-flash your project before using it.
