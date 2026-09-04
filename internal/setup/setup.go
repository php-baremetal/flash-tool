package setup

import (
	"fmt"
	"io"

	"phpflash/internal/platform"
)

type Runner interface {
	Git(args ...string) error
	Shell(dir, script string, args ...string) error
	Exists(path string) bool
}

type Plan struct {
	IdfPath, IdfVersion           string
	PhpEsp32Path, PhpEsp32Version string
}

const idfRepo = "https://github.com/espressif/esp-idf.git"
const phpEsp32Repo = "https://github.com/php-baremetal/php-esp32.git"

// idfTargets are the ESP-IDF install targets whose toolchains we install. It must cover both
// architectures the firmware supports: esp32s3 pulls the Xtensa toolchain, esp32p4 the RISC-V one.
// Installing only one (e.g. esp32p4) leaves a build for the other family failing with
// "xtensa-esp32s3-elf-gcc ... not found in the PATH".
const idfTargets = "esp32s3,esp32p4"

func fetchOrCheckout(r Runner, repo, path, version string) error {
	if r.Exists(path) {
		if version != "" {
			if err := r.Git("-C", path, "checkout", version); err != nil {
				return err
			}
			// Re-pin the submodules to the checked-out version. Skipping this leaves them at whatever
			// commit was there before -- a classic source of "target mbedcrypto is not built" when a
			// stale mbedtls 4.x sits under an IDF that expects 3.6.x.
			return r.Git("-C", path, "submodule", "update", "--init", "--recursive")
		}
		return nil
	}
	if err := r.Git("clone", "--recursive", repo, path); err != nil {
		return err
	}
	if version != "" {
		if err := r.Git("-C", path, "checkout", version); err != nil {
			return err
		}
		return r.Git("-C", path, "submodule", "update", "--init", "--recursive")
	}
	return nil
}

// Run installs ESP-IDF (clone/checkout + install.sh) then php-esp32 (clone/checkout +
// fetch-php.sh). Idempotent: an existing path is checked out to the requested version
// instead of re-cloned. Errors are attributed to the step that failed.
func Run(r Runner, out io.Writer, plan Plan) error {
	fmt.Fprintln(out, "==> ESP-IDF")
	if err := fetchOrCheckout(r, idfRepo, plan.IdfPath, plan.IdfVersion); err != nil {
		return fmt.Errorf("ESP-IDF step failed: %w", err)
	}
	if err := r.Shell(plan.IdfPath, platform.InstallScript(), idfTargets); err != nil {
		return fmt.Errorf("ESP-IDF install.sh failed: %w", err)
	}

	fmt.Fprintln(out, "==> php-esp32")
	if err := fetchOrCheckout(r, phpEsp32Repo, plan.PhpEsp32Path, plan.PhpEsp32Version); err != nil {
		return fmt.Errorf("php-esp32 step failed: %w", err)
	}
	if err := r.Shell(plan.PhpEsp32Path, "scripts/fetch-php.sh"); err != nil {
		return fmt.Errorf("php-esp32 fetch-php.sh failed: %w", err)
	}
	fmt.Fprintln(out, "done.")
	return nil
}
