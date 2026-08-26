---
eyebrow: 'Docs · Recipes'
lede:    'Add a native PHP function to a project in C: scaffold it with phpflash ext new, edit the generated .c to expose your function, rebuild, and call it from PHP — statically linked into the firmware, no fork of the base image.'
see_also:
  - { href: '../commands/ext.md', meta: 'Commands', label: 'phpflash ext' }
  - { href: 'https://github.com/php-baremetal/php-esp32/blob/master/docs/extensions/custom-extensions.md', meta: 'external', label: 'Custom C extensions (php-esp32)' }
  - { href: './pin-a-php-version.md', meta: 'Recipe', label: 'Pin a PHP version' }
prev: { label: 'Pin a PHP version', href: './pin-a-php-version.md' }
next: { label: 'Bake a .env', href: './bake-a-dotenv.md' }
---

# Scaffold a C extension

When PHP is not the right tool for a job — a display driver, a tight sensor loop, a byte-banging
protocol — you can drop native C into your project and call it from PHP. `phpflash ext new` writes a
working skeleton; you edit it, rebuild, and the new function is callable. This recipe takes a
brand-new function called `temp_c2f` (Celsius to Fahrenheit) from nothing to a value printed by PHP.

## What you need

- A project scaffolded with `phpflash init` (this recipe runs from the project directory).
- The php-esp32 firmware sources installed (`phpflash system-setup`), since the build compiles the
  extension into the image.

## Scaffold the extension

From the project directory, name the extension and let phpflash write the skeleton:

<!-- @code-block language="bash" label="terminal — phpflash ext new" -->
```bash
phpflash ext new temp
# created firmware/exts/temp/temp.c
#
# It exposes temp_hello() and temp_add($a, $b). Add your own functions, then
# `phpflash build` compiles it in -- guard use with function_exists('temp_hello').
```
<!-- @endcode-block -->

The name becomes both the directory `firmware/exts/temp/` and the C symbol `temp_module_entry`, so it
must be a lowercase C identifier (`^[a-z][a-z0-9_]*$`). The generated `temp.c` already contains a
buildable module entry with two example functions to replace.

<!-- @callout variant="note" title="Run it where the config lives" -->
`ext new` writes into `./firmware/exts/`, so run it from the directory holding your
`php-esp32.config.toml`. If it finds no config there it still scaffolds, but prints a note reminding
you to run it from the project directory.
<!-- @endcallout -->

## Edit the generated .c

Open `firmware/exts/temp/temp.c`. It ships with `temp_hello()` and `temp_add($a, $b)` as examples.
Replace `temp_add` with your own function. Below, `temp_c2f($c)` takes one number and returns another,
so it needs its own arginfo block declaring one required argument, and it reads that argument with
`Z_PARAM_DOUBLE`:

<!-- @code-block language="c" label="firmware/exts/temp/temp.c — one real function" -->
```c
#include "php.h"

/* temp_c2f($celsius): float -- convert Celsius to Fahrenheit. */
ZEND_BEGIN_ARG_INFO_EX(arginfo_temp_c2f, 0, 0, 1)
    ZEND_ARG_INFO(0, celsius)
ZEND_END_ARG_INFO()

PHP_FUNCTION(temp_c2f)
{
    double celsius;
    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_DOUBLE(celsius)
    ZEND_PARSE_PARAMETERS_END();
    RETURN_DOUBLE(celsius * 9.0 / 5.0 + 32.0);
}

static const zend_function_entry temp_functions[] = {
    PHP_FE(temp_c2f, arginfo_temp_c2f)
    PHP_FE_END
};

zend_module_entry temp_module_entry = {
    STANDARD_MODULE_HEADER,
    "temp",
    temp_functions,
    NULL, NULL, NULL, NULL, NULL,   /* MINIT, MSHUTDOWN, RINIT, RSHUTDOWN, MINFO */
    "0.1",
    STANDARD_MODULE_PROPERTIES,
};
```
<!-- @endcode-block -->

Two rules keep this linking cleanly:

- **Keep the module-entry name equal to the directory name.** The build derives `temp_module_entry`
  from the directory `temp/`; a mismatch means the generated registration table references a symbol
  that does not exist and the link fails.
- **Give each function the right arginfo.** A function with arguments gets its own `arginfo_*` block;
  a no-argument function passes `NULL` in the table (as the scaffold's `temp_hello` does). Do not point
  a function at another function's arginfo.

<!-- @callout variant="warning" title="Write C, under relaxed flags" -->
The engine headers are C and the extension compiles under the same `-w -fpermissive` flags as the
engine. Warnings are suppressed, so they will not catch mistakes for you. Match the shape of the
bundled `gpio` and `ssd1306` examples.
<!-- @endcallout -->

## Rebuild

`phpflash build` detects `./firmware/exts/`, hands it to the firmware build, and compiles every
extension directory in. There is nothing to enable in the config — the presence of the directory is
enough.

<!-- @code-block language="bash" label="terminal — build and flash" -->
```bash
phpflash build
phpflash flash
phpflash monitor    # watch for: project ext 'temp' registered
```
<!-- @endcode-block -->

On boot the firmware registers each extension and logs `project ext 'temp' registered` to the serial
console — a quick confirmation that your C made it into the image.

## Call it from PHP

The function is now a normal PHP function. Guard optional use with `function_exists()` so a script
that runs on a firmware built without the extension degrades gracefully instead of fatally:

<!-- @code-block language="php" label="project-src/index.php — calling temp_c2f" -->
```php
<?php
if (function_exists('temp_c2f')) {
    echo "20C = ", temp_c2f(20.0), "F\n";   // prints: 20C = 68F
} else {
    echo "temp extension is not built into this firmware\n";
}
```
<!-- @endcode-block -->

## What you'll see

<!-- @code-block language="text" label="serial output (excerpt)" -->
```text
project ext 'temp' registered
--- /sdcard/index.php ---
20C = 68F
```
<!-- @endcode-block -->

## Next

- Need SPI, a filesystem, or networking in the extension? List those ESP-IDF components — one per
  line — in `firmware/exts/temp/idf_requires.txt`; the common hardware components
  (`esp_driver_gpio`, `esp_driver_i2c`, `driver`, `esp_timer`) are already linked.
- For the full firmware-side contract — how the build globs and registers extensions, argument
  handling in depth, and the SSD1306 native-driver worked example — read
  [custom C extensions](https://github.com/php-baremetal/php-esp32/blob/master/docs/extensions/custom-extensions.md)
  in php-esp32.
- The command reference for [`phpflash ext`](../commands/ext.md) documents the flags and the full
  generated skeleton.
