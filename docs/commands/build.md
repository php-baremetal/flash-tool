---
eyebrow: 'Docs · Commands'
lede: 'Turn the project config into a firmware image. build resolves the paths and PHP version, turns the enabled extensions into a deterministic build-flag list, pins the ESP-IDF target from the board family, and drives ESP-IDF into a per-project build tree — then copies the flashable images up into build/.'
see_also:
  - { href: './system-setup.md', meta: 'Commands', label: 'phpflash system-setup' }
  - { href: './flash.md', meta: 'Commands', label: 'phpflash flash' }
  - { href: '../../php-esp32/docs/extensions/porting-status.md', meta: 'Extensions', label: 'Porting status' }
prev: { label: 'phpflash system-setup', href: './system-setup.md' }
next: { label: 'phpflash flash', href: './flash.md' }
---

# `phpflash build`

`build` compiles the firmware for the project in the current directory. It reads
`php-esp32.config.toml`, works out which extensions to compile and which fetch scripts they need,
resolves the toolchain and PHP version, and drives ESP-IDF into an isolated build tree under the
project's own `build/`. On success the flashable images sit in `build/`, ready for
[`phpflash flash`](./flash.md).

<!-- @code-block language="bash" label="terminal — build" -->
```bash
phpflash build
```
<!-- @endcode-block -->

## What it does

<!-- @steps -->
- **Load the config.** Reads `php-esp32.config.toml` (overlaying `php-esp32.config.local.toml` if
  present). Errors out with a pointer to `phpflash init` when there is no config here.
- **Resolve toolchain and version.** Finds ESP-IDF and php-esp32 by the usual precedence
  (flag → config → env → default). The PHP language version is the project's `[php] version` if it
  pins one, otherwise the php-esp32 repo default; a version that is not installed is rejected with the
  list of what is available.
- **Compute the flags.** Turns the enabled extensions into a deterministic `-D<flag>=ON/OFF` list from
  the manifest, and collects the fetch scripts those extensions need.
- **Fetch, then build.** Runs each fetch script (oniguruma, SQLite, OpenSSL, as needed), then invokes
  `idf.py` into `build/compiled`.
- **Collect the images.** Copies the flashable `.bin` files up from the build tree into `build/`.
<!-- @endsteps -->

The `idf.py` invocation it drives, with the deterministic flag list:

<!-- @code-block language="bash" label="the build command build drives" -->
```bash
idf.py -B ./build/compiled -DSDKCONFIG=./build/sdkconfig \
       -DBOARD=<board> -DPHP_VERSION=<version> -DIDF_TARGET=<target> \
       -D<PHP_EXT_*>=ON/OFF ... build
```
<!-- @endcode-block -->

<!-- @callout variant="info" title="Why every flag is passed explicitly" -->
The build tree is reused between builds, so every optional flag is emitted on each run — extensions and
project types as `ON` or `OFF`, `PHP_CPU_FREQ_MHZ` and the storage sizes even when unset. A flag that
was simply dropped would keep whatever value the CMake cache held from a previous build; passing all of
them makes the result depend only on the current config.
<!-- @endcallout -->

## The build tree

The build tree and its generated `sdkconfig` live under the project's own `build/`
(`build/compiled` for the full ESP-IDF tree, `build/sdkconfig` for the config). Because both are
per project, several projects can share one php-esp32 install with isolated, side-by-side builds, and a
build never depends on what a shared directory held before.

<!-- @code-block language="text" label="tree — build output" -->
```text
my-project/
├── php-esp32.config.toml
├── project-src/
└── build/
    ├── compiled/               the full ESP-IDF build tree
    ├── sdkconfig               the generated config (per project)
    ├── php-esp32.bin           the app image
    ├── bootloader.bin
    └── partition-table.bin
```
<!-- @endcode-block -->

On success the app image (the `.bin` at the build root), `bootloader.bin`
(from `compiled/bootloader/`) and `partition-table.bin` (from `compiled/partition_table/`) are copied
up into `build/`. Those three are what [`phpflash flash`](./flash.md) writes.

## Target pinning

`build` pins the ESP-IDF target from the board's family — `-DIDF_TARGET=esp32p4` for a P4 board,
`esp32s3` for an S3 board — rather than letting `idf.py` infer it. Without the pin, `idf.py` guesses the
target from any stray in-source `sdkconfig`, which silently builds the wrong architecture when that file
is left over from a different family (an `esp32p4` sdkconfig while building an S3 board, say). Pinning
makes the target follow the configured board, not whatever config happens to be lying around.

## microSD versus embedded source

The project's `storage_type` decides where the PHP source lives, and `build` encodes that in two flags:

<!-- @tabs labels="microSD, embedded" -->
<!-- @tab index="0" -->

A `microsd` project ships its source on the FAT32 card, so nothing is baked in. `build` passes
`-DPHP_STORAGE_MICROSD=ON` and no embed flag — you copy `project-src/` to the card yourself.

<!-- @endtab -->
<!-- @tab index="1" -->

An `embedded` project bakes `project-src/` into a read-only image in flash. `build` adds
`-DPHP_EMBED_SRC=<absolute project-src>` (the path is made absolute because ESP-IDF runs from its own
build tree). microSD support is compiled in only if the project opts into a writable card with
`[storage] microsd = true`; otherwise `-DPHP_STORAGE_MICROSD=OFF` and the SD drivers are dropped.
`[storage] reserve_kb` pads the flash `storage` partition on top of the source.

<!-- @endtab -->
<!-- @endtabs -->

`build` also translates other config into flags automatically: a non-default `[php] entry` becomes
`-DPHP_ENTRY` (for a framework's nested front controller), a `web-server` project's `[web-server] init`
becomes `-DPHP_WEB_INIT`, a `firmware/exts/` directory with a custom extension becomes
`-DPHP_PROJECT_EXTS_DIR`, a project `.env` is baked in, a full-openssl TLS project ships the host's CA
bundle, and `[network] dns` / `[board] cpu_freq_mhz` / `[store] size_kb` each get their own define.

## Flags

`build` takes only the two path overrides; everything else comes from the config.

<!-- @params -->
<!-- @param name="--idf-path" type="string" -->
ESP-IDF location, overriding config, environment and default (flag → `[esp-idf] path` → `IDF_PATH` →
`~/esp/esp-idf`).
<!-- @endparam -->
<!-- @param name="--php-esp32-path" type="string" -->
php-esp32 location, overriding config, environment and default (flag → `[php-esp32] path` →
`PHP_ESP32_DIR` → `~/esp/php-esp32`).
<!-- @endparam -->
<!-- @endparams -->

## Guardrails

<!-- @callout variant="warning" title="Errors that stop the build early" -->
`build` refuses before touching ESP-IDF when the setup is wrong: no `php-esp32.config.toml` in the
directory (run `phpflash init` first), php-esp32 not found at the resolved path (run
`phpflash system-setup`), or a `[php] version` that is not installed — the last names the versions that
are. Progress is shown as phases with a bar; if the ESP-IDF build itself fails, the full ESP-IDF output
is printed so you can see what happened.
<!-- @endcallout -->

<!-- @callout variant="note" title="Per-project sdkconfig" -->
The generated `sdkconfig` lives under the project's own `build/`, so projects sharing one php-esp32
install never fight over a shared config. If you drive ESP-IDF by hand and switch board or version in
place, remove the stale `sdkconfig` first so it regenerates.
<!-- @endcallout -->

After a successful build, write it to the board with [`phpflash flash`](./flash.md).
