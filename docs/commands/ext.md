---
eyebrow: 'Docs · Commands'
lede:    'Manage a project''s custom C extensions. `phpflash ext new <name>` scaffolds a native extension under ./firmware/exts/, which phpflash build compiles into the firmware and the firmware registers at startup — no fork of the base firmware, no .so files.'
see_also:
  - { href: '../recipes/scaffold-a-c-extension.md', meta: 'Recipe', label: 'Scaffold a C extension' }
  - { href: 'https://github.com/php-baremetal/php-esp32/blob/master/docs/extensions/custom-extensions.md', meta: 'external', label: 'Custom C extensions (php-esp32)' }
  - { href: './build.md', meta: 'Commands', label: 'phpflash build' }
prev: { label: 'phpflash discover', href: './discover.md' }
next: { label: 'phpflash update-certs', href: './update-certs.md' }
---

# phpflash ext

`ext` is a small namespace for a project's own C extensions — native PHP functions written in C and
compiled straight into the firmware. There are no `.so` files and no `dlopen` on this target;
everything is statically linked and registered when the engine starts. The one subcommand today is
`ext new`, which writes a working skeleton you edit and build.

<!-- @callout variant="info" title="What this command does, and what the firmware does" -->
`phpflash ext new` only scaffolds a C file under your project. The compiling, table generation, and
startup registration are the firmware's job — `phpflash build` passes the directory to ESP-IDF, and
php-esp32 globs, compiles, and registers it. The full firmware-side contract is in
[custom C extensions](https://github.com/php-baremetal/php-esp32/blob/master/docs/extensions/custom-extensions.md).
<!-- @endcallout -->

## `phpflash ext new <name>`

Scaffold a new C extension under `./firmware/exts/<name>/`. It creates
`firmware/exts/<name>/<name>.c` from the built-in template, filling `<name>` throughout, and creates
the directory tree if it does not exist. After the next `phpflash build` the extension's functions
are callable from PHP.

<!-- @code-block language="bash" label="terminal — scaffold and build" -->
```bash
phpflash ext new ssd1306      # creates firmware/exts/ssd1306/ssd1306.c
phpflash build                # compiles it into the firmware
```
<!-- @endcode-block -->

The command prints the created path, then a reminder that the extension exposes `<name>_hello()` and
`<name>_add($a, $b)` and that optional use should be guarded with `function_exists('<name>_hello')`.

### The name must be a C identifier

The name becomes **both** the directory on disk and the C symbol `<name>_module_entry`, so it is
validated against `^[a-z][a-z0-9_]*$` — a lowercase letter first, then lowercase letters, digits, and
underscores. A name that does not match is rejected with an explanation rather than producing a file
that will not link.

<!-- @callout variant="note" title="Run it from the project directory" -->
`ext new` writes into `./firmware/exts/`, so run it where your `php-esp32.config.toml` lives. If no
config file is found in the current directory it still scaffolds, but first prints
`note: no php-esp32.config.toml here -- run this from your project directory`. Nothing about the
scaffold depends on the config; the note only guards against writing the file in the wrong place.
<!-- @endcallout -->

### What lands on disk

`ext new <name>` creates one directory holding one `.c` file:

<!-- @code-block language="text" label="tree — after phpflash ext new ssd1306" -->
```text
my-project/
├── php-esp32.config.toml
├── project-src/
│   └── index.php
└── firmware/exts/
    └── ssd1306/
        └── ssd1306.c        defines zend_module_entry ssd1306_module_entry
```
<!-- @endcode-block -->

Every subdirectory of `firmware/exts/` is one extension. You add more `*.c`/`*.h` files to the
directory as needed (the directory is on the include path, so `#include "ssd1306.h"` next to
`ssd1306.c` just works), and list any extra ESP-IDF components — one per line — in
`firmware/exts/<name>/idf_requires.txt`. The common hardware components (`esp_driver_gpio`,
`esp_driver_i2c`, `driver`, `esp_timer`) are already on the link.

### Flags

| Flag | Meaning |
|---|---|
| `--force` | Overwrite an existing `firmware/exts/<name>/<name>.c`. Without it, the command refuses rather than clobber your work. |

## The generated skeleton

The template writes a complete, buildable extension: a module entry named `<name>_module_entry`, a
no-argument starter function, and an example with arguments. For `phpflash ext new myext` it expands
to:

<!-- @code-block language="c" label="firmware/exts/myext/myext.c — generated skeleton" -->
```c
/*
 * myext: a php-esp32 project extension -- custom PHP functions written in C.
 *
 * phpflash compiles this into the firmware from ./firmware/exts/myext/. The module entry must be
 * named `myext_module_entry` (the directory name decides the symbol); the firmware registers it
 * at startup, so the functions below are callable from your PHP. Guard optional use with
 * function_exists('myext_hello'). See docs/custom-extensions.md for the full contract.
 *
 * The common hardware components (esp_driver_gpio, esp_driver_i2c, driver, esp_timer) are already
 * on the link; if you need another ESP-IDF component, add it to myext/idf_requires.txt (one
 * component name per line).
 */
#include "php.h"

/* A starter function -- replace it with your own. */
PHP_FUNCTION(myext_hello)
{
    ZEND_PARSE_PARAMETERS_NONE();
    php_printf("hello from the myext extension\n");
}

/* Example with arguments: myext_add($a, $b) returns $a + $b. */
ZEND_BEGIN_ARG_INFO_EX(arginfo_myext_add, 0, 0, 2)
    ZEND_ARG_INFO(0, a)
    ZEND_ARG_INFO(0, b)
ZEND_END_ARG_INFO()

PHP_FUNCTION(myext_add)
{
    zend_long a, b;
    ZEND_PARSE_PARAMETERS_START(2, 2)
        Z_PARAM_LONG(a)
        Z_PARAM_LONG(b)
    ZEND_PARSE_PARAMETERS_END();
    RETURN_LONG(a + b);
}

static const zend_function_entry myext_functions[] = {
    PHP_FE(myext_hello, NULL)
    PHP_FE(myext_add,   arginfo_myext_add)
    PHP_FE_END
};

zend_module_entry myext_module_entry = {
    STANDARD_MODULE_HEADER,
    "myext",
    myext_functions,
    NULL, NULL, NULL, NULL, NULL,   /* MINIT, MSHUTDOWN, RINIT, RSHUTDOWN, MINFO */
    "0.1",
    STANDARD_MODULE_PROPERTIES,
};
```
<!-- @endcode-block -->

The pieces, top to bottom:

- **`#include "php.h"`** pulls in the engine headers — enough for the function macros, argument
  parsing, and the module-entry struct.
- **`PHP_FUNCTION(name)`** defines a native function. Arguments come off the stack with the
  `ZEND_PARSE_PARAMETERS_*` macros; return values are set with `RETURN_LONG`, `RETURN_TRUE`,
  `RETURN_STRING`, and friends. A no-argument function uses `ZEND_PARSE_PARAMETERS_NONE()`.
- **The arginfo block** (`ZEND_BEGIN_ARG_INFO_EX` … `ZEND_END_ARG_INFO`) declares the parameters to
  the engine; the trailing `2` is the count of required arguments.
- **The function table** (`zend_function_entry[]`) lists every exposed function, each with
  `PHP_FE(function, arginfo)`, terminated by `PHP_FE_END`. A function with no arguments passes `NULL`
  as its arginfo, not another function's block.
- **The module entry** ties it together. The second field, `"myext"`, is the reported name; the five
  `NULL`s are the lifecycle hooks (`MINIT`, `MSHUTDOWN`, `RINIT`, `RSHUTDOWN`, `MINFO`); `"0.1"` is
  the version.

Replace `myext_hello()` and `myext_add()` with your own functions, then `phpflash build`.

## Calling it from PHP

Once built and flashed, the functions are callable from your script. Guard optional use with
`function_exists()` so a script running on a firmware built without the extension degrades gracefully:

<!-- @code-block language="php" label="project-src/index.php — calling myext" -->
```php
<?php
if (function_exists('myext_hello')) {
    myext_hello();                 // prints: hello from the myext extension
    echo myext_add(20, 22), "\n";  // prints: 42
} else {
    echo "myext is not built into this firmware\n";
}
```
<!-- @endcode-block -->

<!-- @callout variant="warning" title="Static only — no runtime loading" -->
There is no `dlopen` on this target. To change an extension you rebuild and reflash; the C is part of
the firmware image. Each successful registration logs `project ext '<name>' registered` on boot,
which confirms the extension made it into the image.
<!-- @endcallout -->

## See also

- The recipe [Scaffold a C extension](../recipes/scaffold-a-c-extension.md) walks the whole loop from
  `ext new` to calling the function from PHP.
- The firmware-side [custom C extensions](https://github.com/php-baremetal/php-esp32/blob/master/docs/extensions/custom-extensions.md)
  doc covers the full contract: how the build discovers the directory, argument handling, `idf_requires.txt`,
  and the SSD1306 worked example.
