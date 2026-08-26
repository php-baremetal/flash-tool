---
eyebrow: 'Docs · Recipes'
lede:    'Choose which PHP language version a project builds against. Several versions (8.3, 8.4, 8.5) live side by side in one php-esp32 install; the [php] version key picks one per project, and leaving it empty follows the firmware default.'
see_also:
  - { href: '../configuration/config-file.md', meta: '8 min' }
  - { href: '../commands/build.md', meta: '6 min' }
  - { href: './bake-a-dotenv.md', meta: '4 min' }
prev: { label: 'The php-esp32 manifest', href: '../configuration/manifest.md' }
next: { label: 'Scaffold a C extension', href: './scaffold-a-c-extension.md' }
---

# Pin a PHP version

An installed php-esp32 carries more than one PHP language version at once. Each is a self-contained directory under `components/php/versions/` — `8.3.33`, `8.4.24`, `8.5.9` — and they coexist: nothing is shared or switched globally. A project picks the one it builds against, and that choice is one line of config. Leave it empty and the project follows the firmware's own default version, so most projects never set it at all.

## What you need

- php-esp32 installed (`phpflash system-setup`), with at least one PHP version under `components/php/versions/`. A stock install ships several.
- A project with a `php-esp32.config.toml` (from `phpflash init`).

## Which key controls it

The version is `[php] version` in `php-esp32.config.toml`. Its value is one of the directory names under `components/php/versions/` — a full `MAJOR.MINOR.PATCH`, for example `8.5.9`. Empty (or absent) means "follow the repo default": `phpflash` reads `default_version` from php-esp32's `php-esp32.toml` and builds that.

<!-- @callout variant="warning" title="Not the same as [php-esp32] version" -->
`[php] version` is the **PHP language** version — a directory under `components/php/versions/`. The unrelated `[php-esp32] version` in the `[php-esp32]` table is a **git ref for the firmware checkout** (which php-esp32 commit to use), not a PHP version. Set the PHP language version under `[php]`.
<!-- @endcallout -->

## Seeing what is installed

The versions available to a project are exactly the directory names under `components/php/versions/` in your php-esp32 checkout — each one is a complete, self-contained version.

<!-- @code-block language="bash" label="list the installed PHP versions" -->
```bash
ls ~/esp/php-esp32/components/php/versions/
# 8.3.33  8.4.24  8.5.9
```
<!-- @endcode-block -->

`phpflash` reads this same list. During `phpflash init`, when more than one version is installed, it adds a **PHP version** prompt, marking the repo default `(default)`; picking the default leaves `[php] version` empty, and picking any other pins that exact string into the config. And if you name a version that is not installed, the build refuses with the list of what is available:

<!-- @code-block language="text" label="an unknown version fails the build" -->
```text
PHP version "8.6.0" is not installed in /home/you/esp/php-esp32 (available: 8.3.33, 8.4.24, 8.5.9)
```
<!-- @endcode-block -->

## Config

Pin the version by setting `[php] version` to one of the installed directory names. This project builds against PHP 8.5.9 regardless of what the firmware's default happens to be:

<!-- @code-block language="toml" label="php-esp32.config.toml" -->
```toml
name         = "my-project"
storage_type = "microsd"
type         = "init-loop"

[board]
target = "esp32-p4-pico"
port   = ""                # empty = autodetect at flash time

[php]
src     = "project-src"
entry   = "index.php"
version = "8.5.9"          # a directory under components/php/versions/;
                           # empty = follow the firmware's default_version
```
<!-- @endcode-block -->

<!-- @callout variant="tip" title="Leave it empty to track the default" -->
If you do not care which minor you build against, omit `version` (or set it to `""`). The project then follows `default_version` from php-esp32's `php-esp32.toml`, so an install that bumps its default carries your project forward with it — no per-project edit.
<!-- @endcallout -->

## Build

`phpflash build` resolves the version — the pinned `[php] version`, or the repo default when it is empty — and passes it straight to the firmware build as `-DPHP_VERSION=<version>`.

<!-- @code-block language="bash" label="build, flash, monitor" -->
```bash
phpflash build && phpflash flash && phpflash monitor
```
<!-- @endcode-block -->

## What you'll see

The engine reports the version it was built with in the boot banner and from `PHP_VERSION`, so the pin is easy to confirm on the serial console:

<!-- @code-block language="text" label="serial output (excerpt)" -->
```text
php-esp32: php_embed_init()...
PHP 8.5.9 on ESP32-P4
```
<!-- @endcode-block -->

Because each version is its own directory, switching is just editing `[php] version` and rebuilding — the versions do not interfere, and one php-esp32 install serves projects pinned to different PHP versions at the same time. For the firmware side of what changes between versions, see the [php-esp32](https://github.com/php-baremetal/php-esp32) docs.

## Next

To add native functions of your own on top of the version you pinned, continue to [Scaffold a C extension](./scaffold-a-c-extension.md).
