---
eyebrow: 'Docs · Commands'
lede:    'Refresh the TLS CA bundle a full-OpenSSL project ships to the device. phpflash build writes the bundle once and never overwrites it; update-certs re-copies the host trust store over the old one, so the device keeps verifying HTTPS peers as root CAs are renewed.'
see_also:
  - { href: 'https://github.com/php-baremetal/php-esp32/blob/master/docs/extensions/openssl.md', meta: 'external', label: 'OpenSSL (php-esp32)' }
  - { href: '../configuration/config-file.md', meta: 'Configuration', label: 'Configuration file' }
  - { href: './build.md', meta: 'Commands', label: 'phpflash build' }
prev: { label: 'phpflash ext', href: './ext.md' }
next: { label: 'Configuration file', href: '../configuration/config-file.md' }
---

# phpflash update-certs

A project that builds the full OpenSSL TLS client verifies the HTTPS peers it talks to against a set
of root CA certificates baked into the firmware image. That bundle comes from your host's system trust
store. `phpflash build` copies it into the project the first time and then leaves it alone;
`update-certs` is how you refresh it — re-copying the current host store over the shipped one to pick
up new or renewed roots, a rotated `certs_source`, or a store that has moved on since the project was
first built.

<!-- @code-block language="bash" label="terminal — refresh the CA bundle" -->
```bash
phpflash update-certs
# updated CA bundle: /etc/pki/tls/certs/ca-bundle.crt -> project-src/certs/ca-bundle.crt
```
<!-- @endcode-block -->

## When you need it

The bundle only exists for projects that build a working TLS client — the full OpenSSL build with the
`tls` setting. That combination is what makes `https://` and `tls://` work from PHP on the device;
without it there is no bundle and nothing to update.

<!-- @code-block language="toml" label="php-esp32.config.toml — a project with a TLS client" -->
```toml
[extensions.openssl]
enabled = true
full    = true      # the real OpenSSL 3.0 libcrypto
tls     = true      # build the ssl:// and tls:// transport, so https:// works
```
<!-- @endcode-block -->

Run `update-certs` when the shipped bundle has gone stale relative to the host: a root CA was renewed
or added, you pointed `certs_source` at a different file, or you simply want the device to trust the
same roots your machine does today. Because `build` writes the bundle only once and never overwrites
it, editing the host store and rebuilding is **not** enough — the refresh is a deliberate, separate
step.

<!-- @callout variant="note" title="First build vs. refresh" -->
`phpflash build` runs `EnsureTLSCerts`: it copies the host bundle into the project *only if one is not
already shipped*, so it never clobbers a bundle you have curated. `update-certs` runs `RefreshTLSCerts`,
which always overwrites. That split keeps builds reproducible while giving you one explicit command to
pull in fresh roots.
<!-- @endcallout -->

## What it writes

`update-certs` copies one file: the host's root-CA bundle into the project, at the path the firmware
reads. The destination is `certs_path` if you set one, otherwise the firmware default
`certs/ca-bundle.crt`, resolved against the project's PHP source folder (`[php] src`, default
`project-src`). So by default it lands at `project-src/certs/ca-bundle.crt`. At build time this path
becomes the firmware define `-DPHP_TLS_CAFILE`, and `main.c` points the transport at it so peers are
verified.

The source it copies from is `[extensions.openssl] certs_source` when set, otherwise the first system
trust store that exists, tried in this order:

| Order | Path | Typical host |
|---|---|---|
| 1 | `/etc/pki/tls/certs/ca-bundle.crt` | Fedora, RHEL |
| 2 | `/etc/ssl/certs/ca-certificates.crt` | Debian, Ubuntu |
| 3 | `/etc/ssl/certs/ca-bundle.crt` | others |
| 4 | `/etc/ssl/cert.pem` | others |

The copy is written `0644` and the existing bundle is removed first, so a read-only host store (often
`0444`) does not leave the shipped file read-only and block the next refresh.

<!-- @callout variant="info" title="Reflash to put the new bundle on the device" -->
`update-certs` only refreshes the file inside the project. The bundle is baked into the firmware
image, so you must `phpflash build` and `phpflash flash` again for the device to actually trust the
updated roots.
<!-- @endcallout -->

## Errors it reports

`update-certs` reads the project config in the current directory and refuses in three cases, each with
a clear message rather than a silent no-op:

- **No project here.** No `php-esp32.config.toml` in the current directory — run it from your project
  directory (or `phpflash init` first).
- **No TLS client.** The project does not build the TLS client (`[extensions.openssl]` `full` + `tls`),
  so there are no certificates to update.
- **Developer-managed path.** `certs_path` is an absolute, on-device path. phpflash cannot manage a
  path that does not resolve inside the project source, so it tells you to update it yourself.
- **No host bundle.** No CA bundle was found on the host and `certs_source` is unset — set
  `[extensions.openssl] certs_source` to a specific file.

## Flags

`update-certs` takes no flags. It reads everything it needs from the project config in the current
directory.
