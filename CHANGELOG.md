# Changelog

## [v0.10.0]

### Added
- **Enum extension settings.** A manifest extension setting can now be an enum (`kind = "enum"` with a
  list of `choices` and a `default`), and `build` emits it as `-D<FLAG>=<value>` rather than the boolean
  `-D<FLAG>=ON/OFF`. The first use is the SQLite API selector that pairs with php-esp32 1.0's `ext/sqlite3`
  support: `[extensions.sqlite] type = "pdo-sqlite" | "sqlite3"` (default `pdo-sqlite`), passed as
  `-DPHP_EXT_SQLITE_API`. Interactive `init` offers the choices; an invalid or absent value falls back to
  the setting's default, and the generated config keeps the chosen value.
- **`build --clean`.** Removes the build directory before building, to clear a poisoned CMake cache — a
  failed configure otherwise caches negative results (e.g. "compiler identification is unknown") and
  repeats them even after the environment is fixed. A build failure now also hints to retry with it.

### Fixed
- **`system-setup` installs both toolchains.** It ran `install.sh esp32p4`, installing only the RISC-V
  toolchain; a later build for an ESP32-S3 board then failed with `xtensa-esp32s3-elf-gcc ... not found
  in the PATH`. It now installs `esp32s3,esp32p4`, covering both the Xtensa and RISC-V toolchains the
  firmware's boards need.
- **`system-setup` re-pins ESP-IDF submodules on checkout.** After checking out an IDF version it now
  runs `git submodule update --init --recursive`, so the submodules match the pinned version instead of
  being left at stale commits (the cause of "target mbedcrypto is not built" when a stray mbedtls 4.x is
  checked out under an IDF that expects 3.6.x).
- **Clear error when the serial port isn't accessible.** `flash` and `monitor` now check the port up
  front (via `access(2)`, without opening it, so the board isn't reset) and, on a permission failure,
  print the fix (`sudo chmod a+rw <port>` for now, `sudo usermod -aG dialout $USER` permanently) instead
  of a cryptic `Path '/dev/ttyACM0' is not readable` from esptool.

## [v0.9.1]

### Added
- **Project name passed to the firmware.** `build` now emits `-DPHP_ESP32_PROJECT_NAME=<name>` (from
  the config's `name`), which the firmware surfaces in `phpinfo()`'s new "PHP Baremetal Infos" table
  alongside the board and the php-esp32 / ESP-IDF versions.

## [v0.9.0]

### Added
- **`phpflash partitions publish`** — writes a `partitions.csv` into the project from the configured
  board's committed table, with guidance comments and a per-board table of sensible `factory` sizes.
  The table is computed from the board's flash size (read from its `sdkconfig.board`) and shows how
  much each `factory` leaves for the generated `storage`/`phpstore` partitions. `--force` overwrites an
  existing file; `--php-esp32-path` points at the firmware checkout. See
  [docs/recipes/custom-partition-table.md](docs/recipes/custom-partition-table.md).
- **Per-project partition table.** When a `partitions.csv` sits next to `php-esp32.config.toml`,
  `build` passes `-DPHP_PARTITIONS_CSV` and the firmware uses it as the fixed-partition spec instead of
  the board's committed table (the generated `storage`/`phpstore` partitions are still appended). The
  build announces `using project partition table`; delete the file to revert to the board default.
- **Numeric extension settings.** The config parser now accepts integer values under
  `[extensions.<name>]` (previously only booleans and strings were kept, so numbers were silently
  dropped). This lets the ESP32-S3 onboard-RGB extension take a pin: `[extensions.s3_onboard_rgb]
  pin = 48`, which `build` passes as `-DPHP_S3_RGB_GPIO` (default 48). The extension itself is read
  from the firmware manifest like any other, so nothing else changed here.

## [v0.8.0]

### Added
- **`[web-server] init`** — a web-server project can name a PHP script the firmware runs once, before
  the HTTP server starts, for one-time setup shared across requests (bring hardware up, seed the
  in-RAM `mem_*` or persistent `store_*` stores). `build` passes it as `-DPHP_WEB_INIT` only for a
  `web-server` project with a non-empty `init`. `init` scaffolds a commented `[web-server]` section.
  See the firmware's
  [mem.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/mem.md).

## [v0.7.0]

### Added
- **`discover --all` understands boards with no microSD slot.** The probe firmware now reports
  `microsd=n/a` for an embedded-only board (e.g. the new `esp32-*-zero`), and `discover` prints
  "microSD: n/a (this board has no card slot)" instead of treating it as an empty slot. Boards and
  their capabilities are still read from the installed php-esp32, so the new boards need no change
  here.

### Fixed
- **`discover --all` board identification when a slotless variant is a candidate.** The final match
  now uses the microSD probe: a mounted card proves the board has a slot, which rules out the
  slotless `-zero` variant and picks the board with a slot -- previously, adding a `-zero` board made
  the non-network candidate count ambiguous and `discover` gave up even though a card had mounted.
  When nothing is detected (no link, no card), it explains that a slotless `-zero` board and an SD
  board with an empty slot look identical, and that inserting a card settles it.

## [v0.6.0]

### Added
- **`[storage] reserve_kb`** — for the firmware's dynamic partition table, `build` passes
  `-DPHP_STORAGE_RESERVE_KB` so an embedded project can pad its flash `storage` partition beyond the
  source (0 = just fit it). A microSD project has no `storage` partition, so the erase step on flash
  is now a no-op there (it was already tolerant of a missing partition).
- **`.env` support** — phpflash reads a project's `.env` (next to `php-esp32.config.toml`), parses it
  (`KEY=VALUE`, `#` comments, `export`, single/double quotes) and bakes it into the firmware, where
  PHP exposes it as `$_ENV` and `getenv()`. Configurable via `[env]` (`enabled`, default on when the
  file exists; `file`, default `.env`). `phpflash init` now adds `.env` to the scaffolded
  `.gitignore`. The values live in flash, not on the microSD -- not secret, but off removable media.
  See the firmware side in
  [environment.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/environment.md).
- **`[store] size_kb`** — sizes the firmware's reboot-persistent key-value store (`store_*`): `build`
  passes `-DPHP_STORE_KB`, and the dynamic partition table turns it into a dedicated NVS partition. 0
  or absent means no persistence. See
  [store.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/store.md).

## [v0.5.0]

### Added
- **Per-project C extensions.** A project's custom C extensions under `./firmware/exts/` are compiled
  into the firmware: `build` passes `-DPHP_PROJECT_EXTS_DIR` when the directory exists (and holds at
  least one extension), otherwise nothing changes. See the firmware side in
  [custom-extensions.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/custom-extensions.md).
- **`phpflash ext new <name>`** — scaffold a custom C extension. It writes
  `firmware/exts/<name>/<name>.c` with a working skeleton (a module entry named `<name>_module_entry`
  plus two example functions), which `build` then compiles in. The name must be a valid lowercase C
  identifier; `--force` overwrites an existing file.

## [v0.4.0]

### Added
- **Per-project PHP version** — `[php] version` in `php-esp32.config.toml` pins the PHP language
  version to build (for example `8.5.9`), one of the versions installed under
  `components/php/versions/`. Empty follows `default_version` in the repo's `php-esp32.toml`, so
  existing configs are unaffected. `phpflash init` offers the installed versions when there is more
  than one, and records the choice only when it differs from the default. A version that isn't
  installed fails the build up front with the list of the ones that are, instead of a late ESP-IDF
  error.

## [v0.3.0]

### Added
- **Chip/board check before flashing** — `phpflash flash` probes the connected chip and refuses to
  write an image built for a different target (for example an ESP32-S3 image onto a P4), with a
  message pointing at `[board].target`. `--force` skips the check, and an inconclusive probe (no board
  yet, a busy port) never blocks — esptool remains the backstop that verifies the chip during the
  write.
- **`storage` partition wipe on a microSD flash** — flashing a `microsd` (non-embedded) project now
  erases a leftover `storage` partition from an earlier embedded build, so a stale in-flash image
  can't mount and shadow the microSD.

### Changed
- **Pinned ESP-IDF target** — the build passes `-DIDF_TARGET` derived from the board's family, so
  `idf.py` never infers the architecture from a stray in-source `sdkconfig` (which could silently
  build for the wrong chip when a config was left over from a different board).
- Errors are reported as a single clean line: the root command sets `SilenceUsage`/`SilenceErrors`, so
  a failed command no longer dumps usage or Cobra's own error trailer.

## [v0.2.0]

### Added

#### Project lifecycle
- `phpflash init [dir]` — scaffold a project (`php-esp32.config.toml`, `.gitignore`,
  `project-src/index.php`). Interactive with a default for every prompt, or `--yes` for defaults;
  `--force` to overwrite. Board, storage/execution modes and optional extensions are read live from
  the installed php-esp32, offering only what the firmware implements *and* the chosen board
  supports.
- `phpflash system-setup` — install the prerequisites: ESP-IDF (clone/checkout + `install.sh`) and
  php-esp32 (clone/checkout + `fetch-php.sh`). Idempotent; updates an existing checkout in place.
- `phpflash build` — derive the deterministic `-D<flag>=ON/OFF` list from php-esp32's manifest, run
  any required fetch scripts, then drive `idf.py` into a per-project `build/` tree (isolated,
  side-by-side builds). Output rendered as phases with a compile progress bar; the raw ESP-IDF log
  is printed on failure.
- `phpflash flash [-p port]` / `phpflash monitor [-p port]` — flash and open the serial console.
  Port autodetection globs `/dev/ttyACM*` then `/dev/ttyUSB*` without opening any device.

#### Storage, networking and TLS
- **Embedded storage** — `storage_type = "embedded"` builds the PHP source into a read-only image
  (`-DPHP_EMBED_SRC`); the board runs with no card. `[storage] microsd` opts an embedded project
  back into a card.
- **Static DNS** — `[network] dns = [...]` passed to the firmware as `-DPHP_NET_DNS`.
- **openssl configuration** — for the full openssl build: `-DPHP_OPENSSL_CONF` from
  `[extensions.openssl] config_path`; `EnsureOpenSSLConf` ships a minimal `openssl.cnf` with the
  source so on-chip key generation works.
- **TLS client certificates** — a full-openssl `tls` project has its CA bundle provisioned from the
  host trust store into `certs_path` (auto-detected source, or `certs_source`), passed as
  `-DPHP_TLS_CAFILE`.
- `phpflash update-certs` — refresh that CA bundle, overwriting it with the current host trust store.

#### Board discovery
- `phpflash discover` — identify the connected board from the board outwards (no project needed):
  list serial ports and their USB bridge, probe the chip with esptool (type, revision, flash, MAC,
  built-in radio), and map it to the supported boards. Reports an unsupported chip plainly.
- `phpflash discover --all` — actively probe a blank board's peripherals by flashing a small
  discovery firmware, built **per candidate board** (reusing that board's `board.c`, so it uses the
  real GPIO wiring), and name the match. Destructive (warns, confirms, reminds to re-flash);
  `-y`/`--yes` skips the prompt.

#### Configuration & integration
- `php-esp32.config.toml` schema: `name`, `storage_type`, `type`, `[board]`, `[esp-idf]`,
  `[php-esp32]`, `[storage]`, `[network]`, `[extensions.<name>]` (bool + string settings), `[php]`.
- `php-esp32.config.local.toml` — optional, git-ignored overlay for machine-specific tweaks.
- Path/version resolution: CLI flag → config → environment (`IDF_PATH`, `PHP_ESP32_DIR`) → default.
- **Zero hard-coded hardware/extension knowledge**: extensions, flags, settings, boards and modes
  are all read from the installed php-esp32 at runtime (`php-esp32.toml`, the per-version
  `manifest.toml`, `board.toml`/`family.toml`). `scripts/check-manifest.py` in php-esp32 keeps the
  manifest in step with the build.

### Packaging
- Single static Go binary; dependencies limited to `spf13/cobra` and `BurntSushi/toml`.
- Version stamped at build time via `-ldflags "-X phpflash/cmd.Version=<tag>"` (`--version`).
- Release workflow builds Linux `amd64`/`arm64`, generates `SHA256SUMS`, and publishes a GitHub
  release on a `v*` tag.

