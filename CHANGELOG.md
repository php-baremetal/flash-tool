# Changelog

## [v0.3.0]

### Added
- **Per-project PHP version** — `[php] version` in `php-esp32.config.toml` pins the PHP language
  version to build (for example `8.5.9`), one of the versions installed under
  `components/php/versions/`. Empty follows `default_version` in the repo's `php-esp32.toml`, so
  existing configs are unaffected. `phpflash init` offers the installed versions when there is more
  than one, and records the choice only when it differs from the default. A version that isn't
  installed fails the build up front with the list of the ones that are, instead of a late ESP-IDF
  error.
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

