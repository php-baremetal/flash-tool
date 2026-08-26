---
eyebrow: 'Docs · Configuration'
lede:    'phpflash holds no hard-coded list of extensions or boards. It reads the installed firmware''s manifest and descriptors at runtime to discover which extensions exist (with their build flags and settings), which chip families and boards there are, and which storage and execution modes each supports — so init only offers what the firmware implements and the board can physically do, and build emits exactly the right flags.'
see_also:
  - { href: './config-file.md', meta: 'Configuration', label: 'Configuration file' }
  - { href: '../recipes/pin-a-php-version.md', meta: 'Recipes', label: 'Pin a PHP version' }
  - { href: 'https://github.com/php-baremetal/php-esp32', meta: 'external', label: 'php-esp32 firmware' }
prev: { label: 'Configuration file', href: './config-file.md' }
next: { label: 'Pin a PHP version', href: '../recipes/pin-a-php-version.md' }
---

# The contract with php-esp32

phpflash and [php-esp32](https://github.com/php-baremetal/php-esp32) are two separate repositories:
php-esp32 is the firmware, phpflash is the tool a PHP developer runs. A deliberate design rule keeps
them decoupled — phpflash holds **no** hard-coded knowledge of extensions or hardware. The extensions,
their build flags and settings, the chip families and boards, the PHP versions, and which storage and
execution modes each supports all come from the installed firmware. phpflash reads that, presents it,
records the choices, and emits the right build flags. Nothing more.

The upshot: add an extension, a setting, a board, a PHP version or a per-type rule in php-esp32, and
phpflash picks it up with no code change. This page is the reference for the descriptor files it reads
and how it turns them into offered choices and a build.

## The files phpflash reads

From an installed php-esp32 tree, phpflash reads four kinds of descriptor. The first names the
defaults, the second (per PHP version) is the extension and mode manifest, and the last two describe
the hardware.

| File | Struct | Provides |
|---|---|---|
| `php-esp32.toml` (repo root) | `Repo` | `default_version`, `default_board`. |
| `components/php/versions/<v>/manifest.toml` | `Manifest` | Per PHP version: the storage and project modes the firmware implements, and every extension with its flag, settings, fetch script, requirements and per-type rules. |
| `boards/<family>/family.toml` | `Family` | The chip family: its display name and ESP-IDF `target`. |
| `boards/<family>/<board>/board.toml` | `Board` | A board: its name, family, the storage and project types its hardware supports, and its network interface. |

A concrete tree, taken from phpflash's own test fixtures:

<!-- @code-block language="text" label="tree — an installed php-esp32 (descriptor files)" -->
```text
php-esp32/
├── php-esp32.toml                       default_version, default_board
├── boards/
│   └── esp32-p4/
│       ├── family.toml                  name + ESP-IDF target for the family
│       └── esp32-p4-pico/
│           └── board.toml               one board's peripherals and modes
└── components/
    └── php/
        └── versions/
            └── 8.3.32/
                └── manifest.toml        the extension + mode manifest, per version
```
<!-- @endcode-block -->

### `php-esp32.toml` — the repo defaults

The root descriptor is tiny: the PHP version and the board to fall back to when a project pins neither.

<!-- @code-block language="toml" label="php-esp32.toml" -->
```toml
default_version = "8.3.32"
default_board   = "esp32-p4/esp32-p4-pico"
```
<!-- @endcode-block -->

When a project's `[php] version` is empty, phpflash builds `default_version`. The installed versions
are just the directory names under `components/php/versions/`, so a build against a version that is not
installed fails with the list of what is.

### `manifest.toml` — extensions and modes, per PHP version

One manifest per PHP version, so different versions can support different extensions. It declares three
things: the storage modes, the project (execution) modes, and the extensions.

<!-- @code-block language="toml" label="components/php/versions/8.3.32/manifest.toml" -->
```toml
schema_version = 1

[[storage_type]]
key = "microsd"
available = true
description = "SD"

[[storage_type]]
key = "embedded"
available = false
description = "compiled in"

[[project_type]]
key = "init-loop"
available = true
description = "loop"

[[project_type]]
key = "web-server"
available = false
description = "http"

[[extension]]
key = "gpio"
description = "GPIO"
required_for = ["init-loop"]

[[extension]]
key = "date"
description = "DateTime"
required_for = []
flag = "PHP_EXT_DATE=ON"

  [[extension.setting]]
  key = "minimal_tz"
  description = "UTC only"
  flag = "PHP_EXT_DATE_MINIMAL_TZ=ON"
  default = false

[[extension]]
key = "sqlite"
description = "PDO SQLite"
required_for = []
flag = "PHP_EXT_SQLITE=ON"
fetch = "scripts/fetch-sqlite.sh"
```
<!-- @endcode-block -->

Each `[[storage_type]]` and `[[project_type]]` is a *mode*, with an `available` flag saying whether the
firmware implements it (embedded and web-server are shown here as not-yet-available in the fixture).
Project-type modes may carry a `flag` — the build define for that execution model.

Each `[[extension]]` carries:

| Field | Meaning |
|---|---|
| `key` | The extension name, matching an `[extensions.<key>]` table in the project config. |
| `description` | Human-readable label shown in `init`. |
| `flag` | The `-D…=ON` build define. **Absent for a core extension** — those are always compiled and never listed as optional. |
| `fetch` | A script `build` runs the first time the extension is enabled (to download an external library). |
| `required_for` | The project types this extension is mandatory for (`gpio` is required for `init-loop`). |
| `requires` | Other extensions it depends on, pulled in transitively when it is enabled. |
| `[[extension.setting]]` | Zero or more settings, each with its own `key`, `description`, `flag`, `default` and optional `fetch`. These become the bool/string keys inside the project's `[extensions.<key>]` table. |

### `family.toml` and `board.toml` — the hardware

A family declares the ESP-IDF target; a board declares what its hardware can actually do. phpflash
discovers families by globbing `boards/*/family.toml`, and a family's boards by globbing
`boards/<family>/*/board.toml` — the directory name is the key.

<!-- @code-block language="toml" label="boards/esp32-p4/family.toml" -->
```toml
schema_version = 1
name   = "ESP32-P4"
target = "esp32p4"
psram  = true
```
<!-- @endcode-block -->

<!-- @code-block language="toml" label="boards/esp32-p4/esp32-p4-pico/board.toml" -->
```toml
schema_version = 1
name = "ESP32-P4-Pico"
family = "esp32-p4"
storage_types = ["microsd", "embedded"]
project_types = ["init-loop", "event-driven"]
```
<!-- @endcode-block -->

| Field | Where | Meaning |
|---|---|---|
| `name`, `target` | family | Display name and the ESP-IDF target the whole family builds for (`esp32p4`, `esp32s3`). |
| `name`, `family` | board | The board's label and the family it belongs to. |
| `storage_types` | board | The storage modes the hardware supports. A board that lists `microsd` has a card slot (`HasMicroSD()`); a `-Zero` board lists only `embedded`. |
| `project_types` | board | The execution modes the hardware supports. A board omits `web-server` when it has no network. |
| `network` | board | The network interface: `ethernet`, `wifi`, or absent (none). If absent, it is inferred as present when the board offers `web-server`. |

<!-- @callout variant="note" title="Peripherals are a board property, not a chip probe" -->
The network interface and the microSD slot are external to the SoC (a PHY, a radio module, a card
socket), so a chip probe cannot see them. They are declared in `board.toml`, which is also why
`phpflash discover --all` flashes a per-board probe firmware to tell apart boards that share the same
silicon.
<!-- @endcallout -->

## From descriptors to offered choices

`phpflash init` never offers a mode the firmware has not implemented, nor one the chosen board cannot
do. Those are two separate facts — one in the manifest, one in `board.toml` — and the offer is their
**intersection**.

<!-- @code-block language="go" label="internal/manifest — OfferedModes (intersection)" -->
```go
// OfferedModes are the storage/project types that are both available in the
// manifest and supported by the board.
func (m *Manifest) OfferedModes(board *Board) (storage, project []Mode) {
    for _, s := range m.StorageTypes {
        if s.Available && contains(board.StorageTypes, s.Key) {
            storage = append(storage, s)
        }
    }
    for _, p := range m.ProjectTypes {
        if p.Available && contains(board.ProjectTypes, p.Key) {
            project = append(project, p)
        }
    }
    return
}
```
<!-- @endcode-block -->

So `web-server` appears in `init` only when the manifest marks it `available` *and* the board lists it
in `project_types`. A P4-Pico (no wired network) omits it; a P4-ETH includes it. Extensions split the
same way per project type:

- **`MandatoryFor(type)`** — extensions whose `required_for` includes the chosen project type. Always
  active, not shown as a toggle (`gpio` for `init-loop`).
- **`OptionalFor(type)`** — extensions that carry a `flag` and are not mandatory for the type. These
  are the multi-select `init` presents, each then prompting for its own settings.

## The "effective" set: what actually gets built

Once a project has picked a type and enabled some optional extensions, `Effective` resolves the final
build. It starts from the type's mandatory extensions, adds the enabled optional ones, and pulls in
each one's `requires` transitively — then collects the sorted `-D…` flag list, the extensions that
were pulled in as dependencies, and the fetch scripts to run.

<!-- @steps -->
1. **Seed the mandatory extensions** for the project type (`required_for`).
2. **Enable each optional extension the project turned on**, and recurse through its `requires` so a
   dependency is enabled even if the project did not name it (a pulled-in key is recorded).
3. **Collect flags** — each active extension's `flag`, plus the `flag` of every setting the project
   switched on. An unknown `requires` key is a hard error.
4. **Collect fetches** — the `fetch` script of each active extension and each enabled setting, so
   `build` downloads external sources the first time they are needed.
5. **Sort** the flags, pulled-in keys and fetches so the result is deterministic.
<!-- @endsteps -->

<!-- @callout variant="info" title="Every optional flag is emitted, ON or OFF" -->
The build does not only pass the `=ON` flags. Every optional flag the manifest knows about that a
project did *not* enable is emitted as `=OFF`, so a build is deterministic regardless of what a shared
build directory held before. Core extensions (no flag) are always compiled and never appear as a
define.
<!-- @endcallout -->

Alongside the extension flags, the build pins the ESP-IDF target from the board's family via
`TargetForBoard` (walking the families to find the one that contains the board), so `idf.py` never
guesses the architecture from a stray in-source `sdkconfig` and builds the wrong chip.

## Where this is assembled: `cmd/context.go`

There is no standalone `phpflash context` command to print this. The resolution lives in
`loadBuildContext` (`cmd/context.go`), which every `build` and `flash` calls to turn the config plus
the manifest into the concrete `idf.py` invocation. In order it:

<!-- @steps -->
1. Loads `php-esp32.config.toml` (overlaying `…local.toml`) and resolves the ESP-IDF and php-esp32
   paths by the flag > config > env > default precedence.
2. Loads `php-esp32.toml` for the default PHP version, then the project's pinned `[php] version` (or
   that default), failing with the installed list if the version is not present.
3. Loads that version's `manifest.toml` and computes the effective `-D` flags and fetch scripts.
4. Pins `-DIDF_TARGET` from the board's family, then appends the embedded-source, entry, web-init,
   project-exts, `.env`, openssl-config, TLS-CA and DNS arguments as the config calls for them.
<!-- @endsteps -->

<!-- @callout variant="tip" title="Inspecting the resolution" -->
Because the whole set is derived, the way to see what a project would build with is `phpflash build`
itself — it runs the fetches and prints the `idf.py` command it drives. There is no separate
context-dump subcommand; `cmd/context.go` is internal wiring, not a user-facing command.
<!-- @endcallout -->

## Why it is shaped this way

Keeping every fact in php-esp32 means the tool never falls out of step with the firmware. A new
extension, board, PHP version or per-type rule added there is offered by `init` and built by `build`
with no phpflash release — and php-esp32's own `scripts/check-manifest.py` keeps the manifest honest
against the actual CMake build. The manifest is the single source of truth; phpflash is its front end.
