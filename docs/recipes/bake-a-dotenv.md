---
eyebrow: 'Docs · Recipes'
lede:    'Ship a project .env next to the config and phpflash compiles its variables into the firmware image at build time, so PHP reads them the ordinary way — as $_ENV and getenv(). One endpoint, one device name, one token per build, with no value hardcoded in the PHP source.'
see_also:
  - { href: '../configuration/config-file.md', meta: '8 min' }
  - { href: 'https://github.com/php-baremetal/php-esp32/blob/master/docs/environment.md', meta: 'external', label: 'environment.md' }
  - { href: './pin-a-php-version.md', meta: '3 min' }
prev: { label: 'Scaffold a C extension', href: './scaffold-a-c-extension.md' }
next: { label: 'Identify an unknown board', href: './identify-an-unknown-board.md' }
---

# Bake a `.env`

A project can carry a `.env` file next to its `php-esp32.config.toml`. When you build, `phpflash` reads that file, compiles its variables into the firmware image, and the running engine exposes them to PHP as `$_ENV` and through `getenv()`. Nothing is hardcoded in your PHP source and nothing has to live as a plaintext file on the microSD — the values travel inside the app image itself.

This is for the configuration a build should carry with it: a device name, an endpoint, a sample rate, a feature flag, a token. The value is fixed at build time and constant for the whole run; you change it by editing `.env` and rebuilding, not at runtime.

## What you need

- A project with a `php-esp32.config.toml` (from `phpflash init`).
- A board flashed with php-esp32.

## The `.env`

Put a `.env` next to `php-esp32.config.toml` in the project root. One `KEY=VALUE` per line. Keys are C-style identifiers (`[A-Za-z_][A-Za-z0-9_]*`); everything after the first `=` is the value.

<!-- @code-block language="ini" label=".env" -->
```ini
# a comment (only at the start of a line)
DEVICE_NAME=p4-lab-01
API_BASE=https://api.example.test/v1
SAMPLE_HZ=5
DEBUG=1
GREETING="hello from flash"     # double or single quotes are stripped
export TOKEN=abc123             # a leading `export` is ignored
```
<!-- @endcode-block -->

`phpflash` applies a deliberately small dialect: blank lines and lines starting with `#` are ignored (no inline comments after a value), a leading `export ` is dropped, the split is on the first `=`, a matching pair of surrounding quotes is removed, and inside double quotes `\n \t \" \\` are unescaped. There is no shell expansion — `${VAR}` stays literal — and values are always strings.

## Config

The feature is on by default: if a `.env` sits beside the config, it is baked in with no configuration at all. The `[env]` table exists only to override that default.

<!-- @code-block language="toml" label="php-esp32.config.toml — the [env] knobs (optional)" -->
```toml
[env]
enabled = true      # false turns baking off even when .env exists
file    = ".env"    # env file path, relative to the project (default ".env")
```
<!-- @endcode-block -->

With no `[env]` section, the default applies — baked when `.env` exists, a silent no-op when it does not. `enabled` distinguishes three cases: absent means "on when the file exists", `true` forces it on, `false` turns it off. A missing env file is never an error.

<!-- @callout variant="note" title="init keeps it out of git" -->
`phpflash init` adds `.env` to the project `.gitignore`, so a scaffolded project keeps its environment out of version control by default.
<!-- @endcallout -->

## Build & flash

<!-- @code-block language="bash" label="build, flash, monitor" -->
```bash
phpflash build && phpflash flash && phpflash monitor
```
<!-- @endcode-block -->

`build` parses the `.env` and compiles its entries into the image; `flash` writes that image to the board. On boot with entries present, the firmware logs a line such as `applied 5 env var(s) from .env`.

## Reading it from PHP

The variables are set before the PHP engine starts, so they are present for the whole run — in an `init-loop` sketch's `setup()` and every `loop()` tick, and in every `web-server` request. Values are strings, so cast where you need a number and compare flags as strings.

<!-- @code-block language="php" label="project-src/index.php — reading the baked-in values" -->
```php
<?php
$name  = $_ENV['DEVICE_NAME'] ?? '(unset)';   // "p4-lab-01"
$base  = getenv('API_BASE');                  // "https://api.example.test/v1"
$hz    = (int) ($_ENV['SAMPLE_HZ'] ?? '1');   // "5"  -> 5
$debug = ($_ENV['DEBUG'] ?? '0') === '1';     // "1"  -> true

echo "device: $name\n";
echo "endpoint: $base\n";
echo "sampling at {$hz} Hz, debug " . ($debug ? "on" : "off") . "\n";
```
<!-- @endcode-block -->

## What you'll see

The boot log confirms the count of variables applied, then your script reads them back:

<!-- @code-block language="text" label="serial output (excerpt)" -->
```text
php-esp32: applied 5 env var(s) from .env
PHP 8.5.9 on ESP32-P4
device: p4-lab-01
endpoint: https://api.example.test/v1
sampling at 5 Hz, debug on
```
<!-- @endcode-block -->

To change a value, edit `.env` and rebuild — the new value is compiled in on the next `phpflash build`. There is no runtime API to set these from PHP; they are constants of the build.

<!-- @callout variant="info" title="Baked-in is not encrypted" -->
The values are compiled into the chip's internal flash, not the microSD. They are **not secret**: anyone who can read the flash (physical access plus `esptool`) can recover them. Baking them in keeps them off removable media and raises the bar over a plaintext file on the card, but treat a baked `.env` as configuration, not a vault. For real confidentiality use the SoC's flash encryption and secure boot. The full firmware reference is in [environment.md](https://github.com/php-baremetal/php-esp32/blob/master/docs/environment.md).
<!-- @endcallout -->

## Next

To identify a board before you flash a build onto it, continue to [Identify an unknown board](./identify-an-unknown-board.md).
