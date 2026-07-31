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

func fetchOrCheckout(r Runner, repo, path, version string) error {
	if r.Exists(path) {
		if version != "" {
			return r.Git("-C", path, "checkout", version)
		}
		return nil
	}
	if err := r.Git("clone", "--recursive", repo, path); err != nil {
		return err
	}
	if version != "" {
		return r.Git("-C", path, "checkout", version)
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
	if err := r.Shell(plan.IdfPath, platform.InstallScript(), "esp32p4"); err != nil {
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
