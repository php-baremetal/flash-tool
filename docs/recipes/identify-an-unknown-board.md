---
eyebrow: 'Docs · Recipes'
lede:    'You have an unlabeled ESP32 board and the chip alone cannot tell which model it is. phpflash discover --all flashes a small probe firmware, built per candidate board, brings the peripherals up on each, and prints a boxed verdict with the exact -DBOARD= value to build with.'
see_also:
  - { href: '../commands/discover.md', meta: '6 min' }
  - { href: '../../getting-started/quick-start.md', meta: '10 min', label: 'Quick start' }
  - { href: './pin-a-php-version.md', meta: '3 min' }
prev: { label: 'Bake a .env', href: './bake-a-dotenv.md' }
next: { label: 'Customize the partition table', href: './custom-partition-table.md' }
---

# Identify an unknown board

Two boards in the same chip family share the same silicon, so probing the chip cannot tell a P4-ETH from a P4-Pico: the difference is board-level wiring — whether there is an Ethernet PHY, whether there is a microSD slot. `phpflash discover --all` settles it by actively probing those peripherals: it flashes a small discovery firmware, built once per candidate board so it uses each board's real GPIO wiring, brings up the peripherals, and names the match.

## What you need

- The unknown board, plugged in over a USB cable that carries **data**, not just power.
- php-esp32 installed (`phpflash system-setup`) — `discover --all` builds the probe firmware from it.
- If the board might have Ethernet, a network cable connected: the probe identifies the board by bringing the link *up*, so it needs a live link and DHCP.

<!-- @callout variant="warning" title="--all overwrites the app on the board" -->
`discover --all` flashes a probe firmware over whatever is currently on the board. It warns, asks first (`-y`/`--yes` skips the prompt), and reminds you to re-flash your project afterward with `phpflash flash`.
<!-- @endcallout -->

## Look at the chip first

Plug the board in and run plain `discover`. It lists the serial ports and their USB bridge, probes the chip with esptool, and maps it to the supported boards in that family — enough to know the family and get a paste-ready `[board] target` line, but not which model within the family:

<!-- @code-block language="bash" label="terminal — discover" -->
```bash
phpflash discover
```
<!-- @endcode-block -->

The chip and its built-in radio come straight from the silicon; the `network` and `microSD` columns are what each board *model* declares in its `board.toml`, since the chip cannot be probed for them. When the family has more than one model, that is the ambiguity `--all` resolves.

## Probe the peripherals

Run `discover --all`. It orders network-capable candidates first (a link coming up is the clearest discriminator), builds and flashes the discovery firmware for each candidate wiring in turn, and reads back what actually came up — Ethernet, microSD, PSRAM:

<!-- @code-block language="bash" label="terminal — discover --all" -->
```bash
phpflash discover --all      # add -y to skip the confirmation prompt
```
<!-- @endcode-block -->

Each candidate prints a short probe report. When a board's own wiring brings its network up, that is the board, and the run stops there and prints the verdict:

<!-- @code-block language="text" label="discover --all output (verdict)" -->
```text
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

## Read the verdict

The boxed block is the answer. `THIS BOARD IS` names the model, and `build with: -DBOARD=<key>` gives the exact board key. Put that key into a project as `[board] target`:

<!-- @code-block language="toml" label="php-esp32.config.toml" -->
```toml
[board]
target = "esp32-p4-eth"      # the key from the verdict box
```
<!-- @endcode-block -->

## When it cannot decide

The probe is honest about ambiguity. If nothing came up — no Ethernet link, no card — it cannot separate a slotless `-zero` board from a board with an empty slot, and it says so rather than guessing:

<!-- @code-block language="text" label="discover --all output (undecided)" -->
```text
Couldn't uniquely identify the board -- the probe results are above.
If it has Ethernet, connect the cable (the probe needs the link up) and retry.
If it has a microSD slot, insert a card and retry; if it has neither
network nor a card slot, it's the slotless variant (e.g. -zero).
```
<!-- @endcode-block -->

Insert a card (a mounted card rules out the slotless variant) or connect the network cable and run it again. A single non-network candidate in the family is decided even without a card, since there is nothing else it could be.

## After identifying

The discovery firmware is still on the board, so re-flash your project — `phpflash flash`, or `phpflash build && phpflash flash` if you have not built it yet. From here, continue to the [design reference](../reference/design.md) for how phpflash reads php-esp32.
