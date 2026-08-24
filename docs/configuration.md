# Configuration reference

Every option in a project's `php-esp32.config.toml`, what it means, its type and default, and where it
applies. `phpflash init` writes this file; the other commands read it. If you scaffold with `init` you
rarely touch it by hand, but this is the full picture.

A minimal file has a name, a storage type, an execution type, and a board:

```toml
name         = "my-project"
storage_type = "microsd"
type         = "init-loop"

[board]
target = "esp32-p4-pico"
```

Everything else has a default and can be omitted. The sections below are in the order `init` writes
them.

## Top level

| Key | Type | Default | Meaning |
|---|---|---|---|
| `name` | string | the directory name | The project name. Cosmetic; used in scaffolding and messages. |
| `storage_type` | string | `microsd` | Where the PHP source lives: `microsd` (read from a FAT card) or `embedded` (baked into a read-only flash image). Limited to what the board supports. |
| `type` | string | `init-loop` | The execution model: `init-loop` (run the script, then `setup()` once and `loop($tick)` repeatedly), `web-server` (an HTTP server runs PHP per request; needs a networked board), or `event-driven` (reserved). |

## `[board]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `target` | string | `esp32-p4-pico` | The board key, for example `esp32-p4-pico`, `esp32-p4-eth`, `esp32-s3-eth`. Its family decides the ESP-IDF target (an `esp32-s3-*` board builds for `esp32s3`). `phpflash discover` prints the key for a connected board. |
| `port` | string | `""` (autodetect) | Serial port for flash and monitor, for example `/dev/ttyACM0`. Empty means autodetect at flash time. Best kept in the local override, not the committed config. |
| `cpu_freq_mhz` | int | `0` (board default) | Override the CPU clock. `0` or absent uses the board default (360 MHz on the P4). On a P4 the choices are 360 and 400; 400 on a pre-3.0 chip is experimental and can be unstable. |

## `[esp-idf]` and `[php-esp32]`

Where the toolchain and the firmware sources live, and which version to use. Both keys can be empty,
in which case the default applies. See [Path and version resolution](#path-and-version-resolution) for
the full precedence.

| Table | Key | Type | Default | Meaning |
|---|---|---|---|---|
| `[esp-idf]` | `path` | string | `~/esp/esp-idf` | The ESP-IDF checkout. |
| `[esp-idf]` | `version` | string | the version php-esp32 targets | A git ref (for example `v5.5.5`) `system-setup` checks out. |
| `[php-esp32]` | `path` | string | `~/esp/php-esp32` | The php-esp32 firmware checkout. |
| `[php-esp32]` | `version` | string | the default branch | A git ref for the firmware. |

## `[storage]`

Only meaningful for an `embedded` project, and written by `init` only in that case.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `microsd` | bool | `false` | For an `embedded` project, also mount a microSD for writable data alongside the baked-in source. Off means the SD drivers are not even compiled in. A `microsd` project always has the card, so this key does not apply to it. |

## `[network]`

For a board with a network. Optional.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `dns` | list of strings | `[]` (use DHCP) | Static DNS servers the firmware applies after DHCP, for example `["1.1.1.1", "8.8.8.8"]`. Empty means use whatever DHCP hands out. Relevant when a name lookup is needed, such as the HTTPS client. |

## `[web-server]`

For a `web-server` project. Optional.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `init` | string | `""` (none) | A PHP script within `[php] src` the firmware runs **once**, before the HTTP server starts, for one-time setup shared across requests -- bringing hardware up through a C extension, or seeding the in-RAM `mem_*` / persistent `store_*` stores. Its output goes to the console; a failure is logged but non-fatal. A no-op for non-`web-server` projects. See the firmware's [mem.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/mem.md). |

## `[php]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `src` | string | `project-src` | The folder holding the PHP source. This is the deployable: copied to the card, or baked into the embedded image. |
| `entry` | string | `index.php` | The entry file within `src`. A framework points this at its front controller, for example `public/index.php`. |
| `version` | string | `""` (repo default) | The PHP language version to build, one of the directories under `components/php/versions/` (for example `8.5.9`). Empty follows `default_version` in the repo's `php-esp32.toml`. `init` offers the installed versions when there is more than one, and pins this only when you pick a non-default one. An unknown version fails the build with the list of what is installed. |

## `[extensions.<name>]`

One table per enabled optional extension. The always-on extensions (`standard`, `pcre`, `hash`,
`json`, `spl`, `reflection`, `random`, `gpio`) are always compiled and are not listed here. Each table
has `enabled = true` plus any settings that extension declares:

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

An extension with `requires` pulls its dependencies in automatically. Enabling one that needs an
external library (SQLite, oniguruma, OpenSSL) makes `build` fetch the sources the first time.

### Optional extensions and their settings

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
| `openssl` | `no_load_config` | bool | `false` | Full build only: do not read an `openssl.cnf` at startup. See [openssl.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/openssl.md). |
| `openssl` | `certs_path` | string | `certs/ca-bundle.crt` | Where the CA bundle is shipped (relative to the source folder, or an absolute on-device path you manage). Used by the TLS client. |
| `openssl` | `certs_source` | string | auto-detected system store | The host trust store to copy into `certs_path`. |
| `openssl` | `config_path` | string | `openssl.cnf` | Full build only: where the `openssl.cnf` is shipped and read. Relative is written under the source folder; absolute is a path you provide. |

The full openssl reference, including the two config modes and the TLS-client requirements, is in the
firmware's [openssl.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/openssl.md). The
per-extension flash cost is in
[footprint.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/footprint.md).

## Local overrides: `php-esp32.config.local.toml`

If a `php-esp32.config.local.toml` sits next to the config, every command overlays it: keys it sets
win, everything else keeps the committed value. It is for machine-specific tweaks that should not be
committed, typically the serial port or a local toolchain path:

```toml
[board]
port = "/dev/ttyACM1"

[esp-idf]
path = "/opt/esp-idf"
```

It is optional, `init` never creates it, and the scaffolded `.gitignore` lists it, so it stays out of
version control.

## Path and version resolution

For the `[esp-idf]` and `[php-esp32]` paths and versions, the highest available source wins:

1. **CLI flag**: `--idf-path`, `--php-esp32-path`, `--idf-version`, `--php-esp32-version`.
2. **`php-esp32.config.toml`**: the `path` and `version` keys above (a `local.toml` overlays these).
3. **Environment**: `IDF_PATH`, `PHP_ESP32_DIR`.
4. **Default**: `~/esp/esp-idf` and `~/esp/php-esp32`.

## A full example

Every option at once, for reference. Real configs are much shorter, since almost everything defaults.

```toml
name         = "sensor-gateway"
storage_type = "microsd"
type         = "web-server"

[board]
target       = "esp32-s3-eth"
port         = ""
cpu_freq_mhz = 0

[esp-idf]
path    = ""
version = "v5.5.5"

[php-esp32]
path    = ""
version = ""

[network]
dns = ["1.1.1.1", "8.8.8.8"]

[extensions.ctype]
enabled = true

[extensions.mbstring]
enabled = true
onig    = true

[extensions.session]
enabled = true

[extensions.opcache]
enabled = true

[extensions.openssl]
enabled      = true
full         = true
tls          = true
certs_source = "/etc/ssl/certs/ca-certificates.crt"

[php]
src     = "project-src"
entry   = "public/index.php"
version = "8.5.9"              # omit or leave empty to follow the repo default
```
