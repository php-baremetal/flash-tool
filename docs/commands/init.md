---
eyebrow: 'Docs · Commands'
lede: 'Scaffold a PHP-on-ESP32 project: a config file, a project-src/ folder with a starter index.php, and a .gitignore. Interactive by default, driven by the installed php-esp32 manifest so it only offers what the firmware can actually build.'
see_also:
  - { href: './system-setup.md', meta: 'Commands', label: 'phpflash system-setup' }
  - { href: './build.md', meta: 'Commands', label: 'phpflash build' }
  - { href: '../../php-esp32/docs/getting-started/quick-start.md', meta: 'Getting started', label: 'Quick start' }
prev: { label: 'Your first project', href: '../getting-started/first-project.md' }
next: { label: 'phpflash system-setup', href: './system-setup.md' }
---

# `phpflash init [dir]`

`init` scaffolds a new project in `dir`, or in the current directory if `dir` is omitted. A *project*
is a config file plus a source folder: the config picks the board, the storage and execution models,
the PHP version and the extensions to compile; `project-src/` holds your PHP. `init` writes both,
along with a starter script and a `.gitignore`, and leaves you ready to `phpflash build`.

<!-- @code-block language="bash" label="terminal — init" -->
```bash
phpflash init my-project                       # interactive
phpflash init my-project --board esp32-s3-eth  # preselect the board
phpflash init . --yes                          # scaffold here, all defaults, no prompts
cd my-project
```
<!-- @endcode-block -->

## What it writes

Three things land on disk. `project-src/` is the deployable, kept apart from the config and the build
output so it is easy to reason about and easy to version.

<!-- @code-block language="text" label="tree — scaffolded project" -->
```text
my-project/
├── php-esp32.config.toml   the project config
├── .gitignore              ignores build/, sdkconfig, vendor/, the local override, .env
└── project-src/
    └── index.php           the starter script (hello or blink)
```
<!-- @endcode-block -->

- **`php-esp32.config.toml`** — the project config, rendered from your answers. It is meant to be read
  and edited; see [the config file](#the-config-file) below.
- **`project-src/index.php`** — the entry script, filled from the `hello` or `blink` starter. The
  folder name comes from `[php] src` (default `project-src`) and the file name from `[php] entry`
  (default `index.php`). This folder is exactly what gets copied to a microSD, or baked into the image
  for an `embedded` project.
- **`.gitignore`** — excludes the build tree and machine-local files.

<!-- @code-block language="text" label=".gitignore" -->
```text
/build/
/sdkconfig
/project-src/vendor/
/php-esp32.config.local.toml
/.env
```
<!-- @endcode-block -->

<!-- @callout variant="info" title="init refuses to clobber an existing config" -->
If `php-esp32.config.toml` already exists in the target directory, `init` stops rather than overwrite
it. Pass `--force` to remove the old config first and re-scaffold.
<!-- @endcallout -->

## The interactive flow

`init` is interactive, with a default at every step. It reads the installed php-esp32 through its
manifest, so the board list, the storage and execution modes, the PHP versions and the optional
extensions are exactly what the firmware you have installed can build — nothing offered that would not
compile. The prompts, in order:

<!-- @steps -->
- **Project name** — defaults to the directory name.
- **Chip family, then board** — the family list comes from the manifest; picking the board fixes the
  ESP-IDF target for you. The `--board` value seeds the board default.
- **Storage type** — `microsd` or `embedded`, limited to what the selected board declares it supports.
- **Project type** — the execution model, one of the modes the manifest offers for the board
  (`init-loop`, `web-server`, `event-driven`); a board with no network will not offer `web-server`.
- **PHP version** — asked only when more than one version is installed. Choosing the marked default
  leaves the config's version empty (the project follows the repo default); any other choice pins it.
- **Optional extensions** — a multi-select of the extensions the manifest marks optional for the chosen
  project type. Each one you enable then asks about its own boolean settings (for example `mbstring`
  offers oniguruma for `mb_ereg`). Always-on extensions are not listed.
- **Serial port** — leave empty to autodetect at flash time.
- **Starter** — a `hello` page or a `blink` sketch to write into `project-src/index.php`.
<!-- @endsteps -->

<!-- @callout variant="note" title="When php-esp32 is not installed yet" -->
If the manifest cannot be read at `--php-esp32-path`, `init` prints
`note: php-esp32 not installed; board and extensions default -- configure after system-setup`, keeps
the flag/default board, and skips the board, mode, version and extension prompts. Run
[`phpflash system-setup`](./system-setup.md) first, or fill those fields in by hand afterward.
<!-- @endcallout -->

## Flags

Every prompt has a corresponding default, so the flags below are for non-interactive use and
preselection. `--yes` accepts every default and asks nothing.

<!-- @params -->
<!-- @param name="--name" type="string" -->
Project name written to the config. Defaults to the base name of the target directory.
<!-- @endparam -->
<!-- @param name="--board" type="string" -->
Board / target to preselect, for example `esp32-s3-eth`. Defaults to `esp32-p4-pico`. In interactive
mode this seeds the board default; with `--yes` it is used as-is.
<!-- @endparam -->
<!-- @param name="--php-esp32-path" type="string" -->
Path to the installed php-esp32, used to read the board, mode, version and extension lists. Defaults
to the `PHP_ESP32_DIR` environment variable, then `~/esp/php-esp32`.
<!-- @endparam -->
<!-- @param name="--yes" type="bool" -->
Accept all defaults and ask nothing. Produces a `microsd`, `init-loop` project with no optional
extensions, an empty serial port and the `hello` starter, on the `--board` board.
<!-- @endparam -->
<!-- @param name="--force" type="bool" -->
Overwrite an existing `php-esp32.config.toml` (removed before re-scaffolding).
<!-- @endparam -->
<!-- @endparams -->

## The config file

`init` renders a commented `php-esp32.config.toml` from your answers. A minimal one looks like this:

<!-- @code-block language="toml" label="php-esp32.config.toml" -->
```toml
name = "my-project"
storage_type = "microsd"   # where the PHP source lives
type = "init-loop"         # execution model

[board]
target = "esp32-p4-pico"
port   = ""                # empty = autodetect at flash time

[esp-idf]
path    = ""
version = ""

[php-esp32]
path    = ""
version = ""

[php]
src     = "project-src"    # PHP source folder (copied to the microSD / embedded)
entry   = "index.php"      # entry file within src
version = ""               # PHP version to build (e.g. "8.5.9"); empty = the repo default
```
<!-- @endcode-block -->

Each optional extension you enabled becomes an `[extensions.<key>]` table with `enabled = true` plus
any settings you turned on. The `[esp-idf]` and `[php-esp32]` tables (each `path` + `version`) let a
project pin a specific toolchain or firmware checkout; leave them empty to use the `system-setup`
defaults. The rendered file also carries commented `[env]`, `[store]` and `[web-server]` sections you
can uncomment.

<!-- @callout variant="tip" title="Keep machine-specific settings out of git" -->
For tweaks that should not be committed — a serial port, a local toolchain path — put them in a
`php-esp32.config.local.toml` next to the main file. Every command overlays that file when present,
and the scaffolded `.gitignore` already excludes it. `init` never creates it.
<!-- @endcallout -->

After `init`, the usual next step is [`phpflash build`](./build.md).
