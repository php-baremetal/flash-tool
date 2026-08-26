---
eyebrow: 'Docs · Getting started'
lede: 'Go from nothing to PHP running on the board: install the toolchain once, scaffold a project, then build, flash, and monitor. A project is just a config file and a project-src/ folder.'
see_also:
  - { href: './installation.md', meta: 'Getting started', label: 'Installation' }
  - { href: '../commands/init.md', meta: 'Commands', label: 'phpflash init' }
  - { href: 'https://github.com/php-baremetal/php-esp32', meta: 'external', label: 'php-esp32' }
prev: { label: 'Installation', href: './installation.md' }
next: { label: 'phpflash init', href: '../commands/init.md' }
---

# Your first project

This page takes a board from nothing to running your own PHP: install the toolchain once, scaffold a
project, then build, flash, and watch it boot. It assumes `phpflash` is already on your `PATH` — if
not, start with [Installation](./installation.md).

## The mental model

A *project* is two things: a config file, `php-esp32.config.toml`, and a `project-src/` folder that
holds your PHP. The config picks the board, where the code lives, how it runs, and which extensions to
compile; `project-src/` is the deployable — it is what gets copied to a microSD card or baked into the
firmware image. `phpflash init` writes both for you, plus a `.gitignore`.

<!-- @code-block language="text" label="tree — what init scaffolds" -->
```text
my-project/
├── php-esp32.config.toml   the project config
├── .gitignore              ignores build/, sdkconfig, vendor/, the local override, .env
└── project-src/
    └── index.php           your entry script (the chosen starter)
```
<!-- @endcode-block -->

## The whole sequence

<!-- @steps -->
1. **Install the toolchain** — `phpflash system-setup`, once per machine. It installs ESP-IDF and the php-esp32 firmware sources.
2. **Scaffold a project** — `phpflash init my-project` answers a short series of prompts and writes the config plus a starter `project-src/index.php`.
3. **Enter the project** — `cd my-project`.
4. **Write your PHP** — edit `project-src/index.php` (or point `[php] entry` at a front controller).
5. **Build** — `phpflash build` turns the enabled extensions into build flags and drives ESP-IDF into the project's own `build/`.
6. **Flash** — `phpflash flash` builds if needed, checks the connected chip matches the project's board, then writes it.
7. **Monitor** — `phpflash monitor` opens the serial console. Watch the boot log; leave with `Ctrl-]`.
<!-- @endsteps -->

## 1. Install the toolchain (once)

Before the first project, install the cross-toolchain and the firmware sources. This is a one-time
step per machine:

<!-- @code-block language="bash" label="terminal — system-setup" -->
```bash
phpflash system-setup
```
<!-- @endcode-block -->

It clones ESP-IDF and runs its installer, then clones php-esp32 and runs `scripts/fetch-php.sh` to
download and patch the PHP source. It is idempotent — safe to re-run. Everything lands under `~/esp`
by default.

## 2. Scaffold a project

<!-- @code-block language="bash" label="terminal — init" -->
```bash
phpflash init my-project
```
<!-- @endcode-block -->

`init` is interactive, with a default at every step, and it reads your installed php-esp32 so it only
offers what the firmware can actually build. In order, it asks:

<!-- @steps -->
1. **Project name** — defaults to the directory name.
2. **Chip family, then board** — the family list and the boards within it come from php-esp32's `boards/`. Picking the board fixes the ESP-IDF target for you.
3. **Storage type** — `microsd` or `embedded`, limited to what the selected board supports.
4. **Project type** — the execution model: `init-loop`, `web-server`, or `event-driven`, limited to what the board supports.
5. **PHP version** — offered only when more than one version is installed. Choosing the default leaves the config's version empty (the project follows the repo default); picking another pins it.
6. **Optional extensions** — a multi-select of the extensions optional for the chosen project type. Each one you enable then asks, one by one, about its own settings.
7. **Serial port** — leave it empty to autodetect at flash time.
8. **Starter** — a `hello` page or a `blink` sketch to place in `project-src/index.php`.
<!-- @endsteps -->

The flags let you skip or steer the prompts: `--yes` accepts every default and asks nothing, `--board
<id>` preselects a board, `--name <name>` sets the project name, and `--force` overwrites an existing
`php-esp32.config.toml`.

<!-- @callout variant="note" title="If php-esp32 is not installed yet" -->
When `init` cannot find an installed php-esp32, it prints
`note: php-esp32 not installed; board and extensions default -- configure after system-setup` and
skips the board and extension prompts. Run `system-setup` first, or fill those keys into the config by
hand afterward.
<!-- @endcallout -->

### The two starters

The final prompt seeds `project-src/index.php` with one of two templates:

<!-- @tabs labels="hello, blink" -->
<!-- @tab index="0" -->

A minimal linear script that runs once and prints to the serial console.

<!-- @code-block language="php" label="project-src/index.php — hello" -->
```php
<?php
echo "Hello from PHP on ESP32\n";
```
<!-- @endcode-block -->

<!-- @endtab -->
<!-- @tab index="1" -->

An Arduino-style `setup()`/`loop()` sketch that blinks an LED on GPIO2. The firmware calls `setup()`
once at boot, then `loop($tick)` repeatedly with an incrementing tick; `delay()` paces it in
milliseconds.

<!-- @code-block language="php" label="project-src/index.php — blink" -->
```php
<?php
// setup()/loop() sketch: blink an LED on GPIO2.
function setup() {
    gpio_mode(2, GPIO_OUTPUT);
}
function loop($tick) {
    gpio_write(2, $tick % 2);
    delay(500);
}
```
<!-- @endcode-block -->

<!-- @endtab -->
<!-- @endtabs -->

### The config it writes

`init` writes `php-esp32.config.toml`. It is meant to be read and edited — a short one looks like this:

<!-- @code-block language="toml" label="php-esp32.config.toml" -->
```toml
name = "my-project"
storage_type = "microsd"   # where the PHP source lives
type = "init-loop"         # execution model

[board]
target = "esp32-p4-pico"
port   = ""                # empty = autodetect at flash time

[php]
src     = "project-src"    # PHP source folder (copied to the microSD / embedded)
entry   = "index.php"      # entry file within src
version = ""               # PHP version to build; empty = the repo default
```
<!-- @endcode-block -->

An enabled optional extension gets its own `[extensions.<name>]` table with `enabled = true` and any
settings you turned on. The scaffold also leaves `[esp-idf]` and `[php-esp32]` tables (each with
`path` and `version`) so a project can pin a specific toolchain or firmware checkout.

<!-- @callout variant="tip" title="Keep machine-specific settings out of git" -->
A sibling `php-esp32.config.local.toml`, if present, is overlaid on top of the main config for
machine-specific tweaks such as a serial port or a local toolchain path. The scaffolded `.gitignore`
already excludes it, along with `build/`, `sdkconfig`, `project-src/vendor/`, and `.env`.
<!-- @endcallout -->

## 3. Enter the project and write your PHP

<!-- @code-block language="bash" label="terminal — cd and edit" -->
```bash
cd my-project
$EDITOR project-src/index.php
```
<!-- @endcode-block -->

Edit the starter, or replace it entirely. For a framework, point `[php] entry` at the front controller
(for example `public/index.php`) instead of the top-level `index.php`.

## 4. Build

<!-- @code-block language="bash" label="terminal — build" -->
```bash
phpflash build
```
<!-- @endcode-block -->

`build` reads the config, turns the enabled extensions into a deterministic `-D<flag>=ON/OFF` list,
runs any fetch scripts an enabled extension needs, then drives ESP-IDF into a build tree under the
project's own `build/`. Because that tree and its `sdkconfig` are per project, several projects can
share one php-esp32 install with isolated, side-by-side builds. On success it prints
`Build complete. Next: phpflash flash`.

## 5. Flash

<!-- @code-block language="bash" label="terminal — flash" -->
```bash
phpflash flash
```
<!-- @endcode-block -->

`flash` builds first if needed, then runs two guardrails around the write. It probes the connected
chip and refuses if its target does not match the project board's family — an S3 image with a P4
plugged in, say — with `--force` to override. And after a `microsd` flash it erases the board's
`storage` partition, so a leftover embedded image from an earlier build cannot mount and shadow the
card. The port comes from `-p`, then the config's `[board].port`, then the first serial device it
finds.

## 6. Monitor

<!-- @code-block language="bash" label="terminal — monitor" -->
```bash
phpflash monitor
```
<!-- @endcode-block -->

`monitor` opens the serial console. Watch the board boot and print your script's output; leave with
`Ctrl-]`. On a networked board the boot log prints the address it came up on, which is where you point
a browser or `curl` for a `web-server` project.

<!-- @callout variant="tip" title="Where to go next" -->
For every flag and the full behaviour of each step, see the command reference — starting with
[`phpflash init`](../commands/init.md). For writing PHP for the board, the extensions, and the porting
details, see the [php-esp32](https://github.com/php-baremetal/php-esp32) docs.
<!-- @endcallout -->
