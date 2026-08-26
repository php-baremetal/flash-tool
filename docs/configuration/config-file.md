---
eyebrow: 'Docs · Configuration'
lede:    'Every option in a project''s php-esp32.config.toml — the file phpflash init writes and the other commands read: the top-level name/storage/type, the board, the toolchain and firmware sources, storage, network, the web-server hook, the PHP source and version, and one table per enabled extension. Plus the local override and how paths and versions resolve.'
see_also:
  - { href: './manifest.md', meta: 'Configuration', label: 'The php-esp32 manifest' }
  - { href: '../getting-started/quick-start.md', meta: '10 min', label: 'Quick start' }
  - { href: 'https://github.com/php-baremetal/php-esp32', meta: 'external', label: 'php-esp32 firmware' }
prev: { label: 'phpflash update-certs', href: '../commands/update-certs.md' }
next: { label: 'The php-esp32 manifest', href: './manifest.md' }
---

# The project config file

Every project is a `php-esp32.config.toml` plus a `project-src/` folder. The config picks the board,
where the PHP source lives, how it runs, and which extensions to compile. `phpflash init` writes this
file interactively; `build`, `flash` and `monitor` read it. If you scaffold with `init` you rarely
touch it by hand, but this is the full picture — every key, its type and default, and where it applies.

Everything except the name, a storage type, an execution type and a board has a default and can be
omitted. A minimal file is four lines and a table:

<!-- @code-block language="toml" label="php-esp32.config.toml — minimal" -->
```toml
name         = "my-project"
storage_type = "microsd"
type         = "init-loop"

[board]
target = "esp32-p4-pico"
```
<!-- @endcode-block -->

The sections below are in the order `init` writes them.

<!-- @callout variant="info" title="phpflash holds no hard-coded hardware knowledge" -->
The boards, the storage and execution modes, the PHP versions, and the extensions with their settings
all come from the installed [php-esp32](https://github.com/php-baremetal/php-esp32) firmware, read at
runtime from its manifest and board descriptors. This page documents the shape of the config; which
values are *valid* for your install is decided by the firmware — see [the php-esp32
manifest](./manifest.md).
<!-- @endcallout -->

## Top level

The three keys that every project sets, at the root of the file.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `name` | string | the directory name | The project name. Cosmetic; used in scaffolding and messages. |
| `storage_type` | string | `microsd` | Where the PHP source lives: `microsd` (read from a FAT card) or `embedded` (baked into a read-only flash image). Limited to what the board supports. |
| `type` | string | `init-loop` | The execution model: `init-loop` (run the script, then `setup()` once and `loop($tick)` repeatedly), `web-server` (an HTTP server runs PHP per request; needs a networked board), or `event-driven` (reserved). |

`storage_type` and `type` are orthogonal, and both are drawn from php-esp32: a mode is offered only
when it is *both* implemented by the firmware *and* supported by the chosen board. That is why a
`-Zero` board (no card slot) offers only `embedded`, and a board with no network never offers
`web-server`.

## `[board]`

The board and how phpflash reaches it over serial.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `target` | string | `esp32-p4-pico` | The board key, for example `esp32-p4-pico`, `esp32-p4-eth`, `esp32-s3-eth`. Its family decides the ESP-IDF target (an `esp32-s3-*` board builds for `esp32s3`). `phpflash discover` prints the key for a connected board. |
| `port` | string | `""` (autodetect) | Serial port for flash and monitor, for example `/dev/ttyACM0`. Empty means autodetect at flash time. Best kept in the local override, not the committed config. |
| `cpu_freq_mhz` | int | `0` (board default) | Override the CPU clock, passed to the firmware as `-DPHP_CPU_FREQ_MHZ`. `0` or absent uses the board default (360 MHz on the P4). On a P4 the choices are 360 and 400; 400 on a pre-3.0 chip is experimental and can be unstable. |

<!-- @callout variant="tip" title="Port autodetection order" -->
When `port` is empty and no `-p` flag is given, phpflash picks the first serial device that exists —
`/dev/ttyACM*` before `/dev/ttyUSB*` — without opening it, and falls back to letting ESP-IDF
autodetect. Because it never opens the port, detection cannot hang on or disturb a device that is not
the board.
<!-- @endcallout -->

## `[esp-idf]` and `[php-esp32]`

Where the toolchain and the firmware sources live, and which version to use. Both keys of both tables
can be empty, in which case the default applies. See [Path and version
resolution](#path-and-version-resolution) below for the full precedence.

| Table | Key | Type | Default | Meaning |
|---|---|---|---|---|
| `[esp-idf]` | `path` | string | `~/esp/esp-idf` | The ESP-IDF checkout. |
| `[esp-idf]` | `version` | string | the version php-esp32 targets | A git ref (for example `v5.5.5`) `system-setup` checks out. |
| `[php-esp32]` | `path` | string | `~/esp/php-esp32` | The php-esp32 firmware checkout. |
| `[php-esp32]` | `version` | string | the default branch | A git ref for the firmware. |

## `[storage]`

Only meaningful for an `embedded` project, and written by `init` only in that case. A `microsd`
project always has the card, so neither key applies to it.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `microsd` | bool | `false` | For an `embedded` project, also mount a microSD for writable data alongside the baked-in source. Off means the SD drivers are not even compiled in. |
| `reserve_kb` | int | `0` (just fit the source) | Pads the embedded `storage` partition: the firmware sizes it to the source plus FAT overhead and adds this many KB on top. Headroom for a larger source, not runtime growth — the image is read-only. |

## `[network]`

For a board with a network. Optional.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `dns` | list of strings | `[]` (use DHCP) | Static DNS servers the firmware applies after DHCP, for example `["1.1.1.1", "8.8.8.8"]`. Empty means use whatever DHCP hands out. Relevant when a name lookup is needed, such as the HTTPS client. Emitted to the build as `-DPHP_NET_DNS`. |

## `[web-server]`

For a `web-server` project. Optional.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `init` | string | `""` (none) | A PHP script within `[php] src` the firmware runs **once**, before the HTTP server starts, for one-time setup shared across requests — bringing hardware up through a C extension, or seeding the in-RAM `mem_*` / persistent `store_*` stores. Its output goes to the console; a failure is logged but non-fatal. A no-op for non-`web-server` projects. |

## `[php]`

The PHP source folder, its entry point, and the language version to build.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `src` | string | `project-src` | The folder holding the PHP source. This is the deployable: copied to the card, or baked into the embedded image. |
| `entry` | string | `index.php` | The entry file within `src`. A framework points this at its front controller, for example `public/index.php`. |
| `version` | string | `""` (repo default) | The PHP language version to build, one of the directories under `components/php/versions/` (for example `8.5.9`). Empty follows `default_version` in the repo's `php-esp32.toml`. `init` offers the installed versions when there is more than one, and pins this only when you pick a non-default one. An unknown version fails the build with the list of what is installed. |

## `[extensions.<name>]`

One table per enabled optional extension. The always-on extensions (`standard`, `pcre`, `hash`,
`json`, `spl`, `reflection`, `random`, `gpio`) are always compiled and are not listed here. Each table
carries `enabled = true` plus any settings that extension declares — a bool per toggle, a string per
path:

<!-- @code-block language="toml" label="php-esp32.config.toml — extension tables" -->
```toml
[extensions.mbstring]
enabled = true
onig    = true          # a bool setting

[extensions.openssl]
enabled      = true
full         = true     # a bool setting
tls          = true     # a bool setting
certs_source = "/etc/ssl/certs/ca-certificates.crt"   # a string setting
```
<!-- @endcode-block -->

An extension that declares `requires` pulls its dependencies in automatically. Enabling one that needs
an external library (SQLite, oniguruma, OpenSSL) makes `build` fetch the sources the first time.

<!-- @callout variant="note" title="Two kinds of setting" -->
phpflash reads a bool setting (like `onig` or `in_memory`) into the extension's flag set, and a string
setting (like `certs_source` or `config_path`) into a separate string map. Both live in the same
`[extensions.<name>]` table as `enabled`. Only the settings the manifest declares for that extension
are meaningful; anything else is ignored.
<!-- @endcallout -->

### Optional extensions and their settings

The full set the firmware ships today. A blank Setting row means the extension has no settings — just
`enabled = true`.

| Extension | Setting | Type | Default | Meaning |
|---|---|---|---|---|
| `ctype` | | | | Character-class checks (`ctype_alpha`, `ctype_digit`). |
| `filter` | | | | `filter_var()` validation and sanitization. |
| `tokenizer` | | | | `token_get_all()`, `PhpToken`, the `T_*` constants. |
| `session` | | | | `session_start()`, `$_SESSION`, save handlers. Needs a writable dir (a microSD) for the files handler. |
| `sqlite` | | | | PDO with the SQLite driver (`pdo` plus `pdo_sqlite`). |
| `date` | | | | The real `ext/date`: `DateTime` and the date/time API. |
| `date` | `minimal_tz` | bool | `false` | Ship a UTC-only timezone database instead of the full one; about 350 KB smaller, no named zones. |
| `mbstring` | | | | Multibyte strings. Built without `mb_ereg` unless `onig` is on. |
| `mbstring` | `no_cjk` | bool | `false` | Drop the legacy CJK codecs (Shift-JIS, EUC, Big5, GB18030); about 755 KB smaller. |
| `mbstring` | `onig` | bool | `false` | Build the `mb_ereg` and `mb_split` family (bundles oniguruma); about 445 KB. |
| `opcache` | | | | Zend OPcache, no JIT. File cache on the microSD by default. |
| `opcache` | `in_memory` | bool | `false` | Keep the bytecode cache in PSRAM instead of on the card. Fastest after warm-up, but the bytecode plus the per-request heap must fit in PSRAM. Small apps only. |
| `openssl` | | | | The `openssl_*` functions. Default is the mbedTLS-backed symmetric subset. |
| `openssl` | `full` | bool | `false` | Build the real `ext/openssl` on a ported OpenSSL 3.0 (RSA, EC, X.509); about 2 MB. |
| `openssl` | `tls` | bool | `false` | Full build only: build the `ssl://` and `tls://` transport so PHP can do HTTPS. Needs a networked board. |
| `openssl` | `no_load_config` | bool | `false` | Full build only: do not read an `openssl.cnf` at startup. |
| `openssl` | `certs_path` | string | `certs/ca-bundle.crt` | Where the CA bundle is shipped (relative to the source folder, or an absolute on-device path you manage). Used by the TLS client. |
| `openssl` | `certs_source` | string | auto-detected system store | The host trust store to copy into `certs_path`. |
| `openssl` | `config_path` | string | `openssl.cnf` | Full build only: where the `openssl.cnf` is shipped and read. Relative is written under the source folder; absolute is a path you provide. |

The full openssl reference — the two config modes and the TLS-client requirements — is in the
firmware's [openssl.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/openssl.md); the
per-extension flash cost is in
[footprint.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/footprint.md). OPcache's
two cache modes have their own page in [OPcache](https://github.com/php-baremetal/php-esp32/blob/master/docs/extensions/opcache.md).

## `[env]` and `[store]`

Two optional tables the scaffold leaves commented, each with its own dedicated page. `[env]` controls
baking a project `.env` into the firmware as `$_ENV` / `getenv()`; `[store]` sizes the
reboot-persistent key-value store.

| Table | Key | Type | Default | Meaning |
|---|---|---|---|---|
| `[env]` | `enabled` | bool | *unset* → on when `.env` exists | `false` turns baking off even when the file is present. |
| `[env]` | `file` | string | `".env"` | Path to the env file, relative to the project. |
| `[store]` | `size_kb` | int | `0` (no persistence) | Add a dedicated NVS partition of this size; `0` or no `[store]` section means `store_available()` is false. |

The full behaviour is on the [build-time environment](https://github.com/php-baremetal/php-esp32/blob/master/docs/storage/environment.md) and
[persistent store](https://github.com/php-baremetal/php-esp32/blob/master/docs/storage/persistent-store.md) pages. The volatile in-RAM store
(`mem_*`) needs no config at all.

## Local overrides: `php-esp32.config.local.toml`

If a `php-esp32.config.local.toml` sits next to the config, every command overlays it: keys it sets
win, everything else keeps the committed value. `init` never creates it, the scaffolded `.gitignore`
lists it, and it is for machine-specific tweaks that should not be committed — typically the serial
port or a local toolchain path.

<!-- @code-block language="toml" label="php-esp32.config.local.toml" -->
```toml
[board]
port = "/dev/ttyACM1"

[esp-idf]
path = "/opt/esp-idf"
```
<!-- @endcode-block -->

The overlay is a straight TOML decode of the local file *over* the already-loaded base config, so it
sets only the fields the local file actually defines. That gives override semantics for scalar and
table keys, and for the extra bool/string keys inside an `[extensions.<name>]` table.

## Path and version resolution

For the `[esp-idf]` and `[php-esp32]` paths and versions, the highest available source wins:

<!-- @steps -->
1. **CLI flag** — `--idf-path`, `--php-esp32-path`, `--idf-version`, `--php-esp32-version`.
2. **`php-esp32.config.toml`** — the `path` and `version` keys above (a `local.toml` overlays these).
3. **Environment** — `IDF_PATH`, `PHP_ESP32_DIR`.
4. **Default** — `~/esp/esp-idf` and `~/esp/php-esp32`.
<!-- @endsteps -->

The first non-empty value in that order is used, so an empty key falls through to the next source
rather than blanking it.

## A full example

Every option at once, for reference. Real configs are much shorter, since almost everything defaults.

<!-- @code-block language="toml" label="php-esp32.config.toml — annotated" -->
```toml
name         = "sensor-gateway"
storage_type = "microsd"          # microsd | embedded
type         = "web-server"       # init-loop | web-server | event-driven

[board]
target       = "esp32-s3-eth"     # board key; its family sets the ESP-IDF target
port         = ""                 # empty = autodetect at flash time
cpu_freq_mhz = 0                  # 0 = board default

[esp-idf]
path    = ""                      # empty = ~/esp/esp-idf
version = "v5.5.5"                # git ref system-setup checks out

[php-esp32]
path    = ""                      # empty = ~/esp/php-esp32
version = ""                      # empty = the default branch

[network]
dns = ["1.1.1.1", "8.8.8.8"]      # static DNS applied after DHCP

[web-server]
init = "init.php"                 # run once before the HTTP server starts

[extensions.ctype]
enabled = true

[extensions.mbstring]
enabled = true
onig    = true                    # build mb_ereg (bundles oniguruma)

[extensions.session]
enabled = true

[extensions.opcache]
enabled = true                    # file cache on the microSD by default

[extensions.openssl]
enabled      = true
full         = true               # real ext/openssl on OpenSSL 3.0
tls          = true               # ssl:// and tls:// transports
certs_source = "/etc/ssl/certs/ca-certificates.crt"

[php]
src     = "project-src"
entry   = "public/index.php"      # a framework's front controller
version = "8.5.9"                 # omit or leave empty to follow the repo default
```
<!-- @endcode-block -->
