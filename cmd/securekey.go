package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"phpflash/internal/config"
	"phpflash/internal/platform"
)

// newSecureKeyCmd generates the Secure Boot v2 signing key for a board and stores it as
// deploys/<MAC>.pem in the project -- one key per physical unit, named by its MAC. The private key
// is a secret (whoever holds it can sign firmware the board accepts), so it is git-ignored and must
// be backed up offline; losing it means that unit can never be updated again.
func newSecureKeyCmd() *cobra.Command {
	var idfPath, phpPath, port, mac, deploysDir string
	c := &cobra.Command{
		Use:   "secure-key",
		Short: "Generate the Secure Boot v2 signing key for a board (deploys/<MAC>.pem)",
		Long: "Generate the Secure Boot v2 signing key for a board and store it as deploys/<MAC>.pem.\n" +
			"One key per physical unit, named by its MAC (read from the connected board, or passed with\n" +
			"--mac). The key is git-ignored and must be backed up offline -- losing it bricks updates for\n" +
			"that unit. It never overwrites an existing key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			idf, _ := resolveDirs(idfPath, phpPath, nil) // board-first: no project config needed

			m := mac
			if m == "" {
				target := port
				if target == "" {
					ports := platform.ListPorts()
					if len(ports) == 0 {
						return fmt.Errorf("no serial device found (looked for /dev/ttyACM*, /dev/ttyUSB*). " +
							"Plug the board in, or pass --mac to skip reading it")
					}
					target = ports[0]
				}
				var err error
				m, err = readBoardMAC(idf, target)
				if err != nil {
					return err
				}
			}

			norm := normalizeMAC(m)
			if len(norm) != 12 {
				return fmt.Errorf("%q is not a 48-bit MAC", m)
			}

			dir := deploysDir
			if dir == "" {
				dir = "deploys"
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", dir, err)
			}
			ensureDeploysGitignore(dir)

			key := filepath.Join(dir, norm+".pem")
			if _, err := os.Stat(key); err == nil {
				fmt.Fprintf(out, "key already exists: %s -- refusing to overwrite (it is irreplaceable)\n", key)
				return nil
			}

			if err := generateSigningKey(idf, key); err != nil {
				return err
			}
			_ = os.Chmod(key, 0o600)

			fmt.Fprintf(out, "generated %s\n", key)
			fmt.Fprintln(out, "  * back it up offline -- it cannot be regenerated; losing it bricks updates.")
			fmt.Fprintln(out, "  * it is git-ignored (deploys/.gitignore) -- never commit a signing key.")
			fmt.Fprintln(out, "  * enable Secure Boot v2 pointing CONFIG_SECURE_BOOT_SIGNING_KEY at it (see the docs).")
			return nil
		},
	}
	c.Flags().StringVar(&idfPath, "idf-path", "", "ESP-IDF path (for esptool / espsecure)")
	c.Flags().StringVar(&phpPath, "php-path", "", "php-esp32 path")
	c.Flags().StringVar(&port, "port", "", "serial port to read the MAC from (default: first found)")
	c.Flags().StringVar(&mac, "mac", "", "use this MAC instead of reading the board (e.g. 28:84:85:67:57:80)")
	c.Flags().StringVar(&deploysDir, "deploys-dir", "", "directory for the key (default: ./deploys)")
	return c
}

// resolveSecureBootKey returns the `-DPHP_SECURE_BOOT_KEY=<abs path>` build arg for a secure_boot
// project, provisioning the key if needed. The key lives at <keysDir>/<MAC>.pem (keysDir defaults to
// ./deploys). To avoid resetting the board on every build, an already-present key is reused: if the
// dir holds exactly one *.pem it is used as-is; otherwise the board's MAC is read to pick/generate
// <MAC>.pem. Generating writes the key (mode 600) and a deploys/.gitignore.
func resolveSecureBootKey(cfg *config.Config, projectDir, idfPath string) (string, error) {
	dir := cfg.SecureBootKeysDir
	if dir == "" {
		dir = "deploys"
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(projectDir, dir)
	}

	pems, _ := filepath.Glob(filepath.Join(dir, "*.pem"))

	var key string
	switch {
	case len(pems) == 1:
		key = pems[0] // one provisioned board for this project -- no need to touch the hardware
	default:
		// 0 keys (first build) or several (pick by the connected board): read the MAC.
		port := cfg.Board.Port
		if port == "" {
			if ports := platform.ListPorts(); len(ports) > 0 {
				port = ports[0]
			}
		}
		if port == "" {
			if len(pems) == 0 {
				return "", fmt.Errorf("secure_boot: no signing key in %s and no board connected to read a MAC "+
					"-- connect the board, or run `phpflash secure-key --mac <MAC>` first", dir)
			}
			return "", fmt.Errorf("secure_boot: several keys in %s and no board connected to pick one "+
				"-- connect the target board", dir)
		}
		mac, err := readBoardMAC(idfPath, port)
		if err != nil {
			return "", fmt.Errorf("secure_boot: %w", err)
		}
		norm := normalizeMAC(mac)
		if len(norm) != 12 {
			return "", fmt.Errorf("secure_boot: %q is not a 48-bit MAC", mac)
		}
		key = filepath.Join(dir, norm+".pem")
		if _, err := os.Stat(key); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("secure_boot: %w", err)
			}
			ensureDeploysGitignore(dir)
			if err := generateSigningKey(idfPath, key); err != nil {
				return "", fmt.Errorf("secure_boot: %w", err)
			}
			_ = os.Chmod(key, 0o600)
			fmt.Fprintf(os.Stderr, "==> secure_boot: generated signing key %s (git-ignored -- back it up offline!)\n", key)
		}
	}

	abs, err := filepath.Abs(key)
	if err != nil {
		return "", err
	}
	return "-DPHP_SECURE_BOOT_KEY=" + abs, nil
}

// readBoardMAC reads the base MAC from the board on `port`, sourcing ESP-IDF's export.sh so esptool
// is on PATH (same approach as the chip probe). Returns the raw "aa:bb:.." string.
func readBoardMAC(idfPath, port string) (string, error) {
	script := ". " + shq(filepath.Join(idfPath, "export.sh")) +
		" >/dev/null 2>&1 && esptool.py -p " + shq(port) + " read_mac"
	outb, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not read the MAC from %s (board connected? port free?): %w\n%s",
			port, err, string(outb))
	}
	for _, line := range strings.Split(string(outb), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "MAC:") {
			if f := strings.Fields(line); len(f) >= 2 {
				return f[1], nil
			}
		}
	}
	return "", fmt.Errorf("no MAC in esptool output:\n%s", string(outb))
}

// generateSigningKey runs espsecure to write an RSA-3072 Secure Boot v2 signing key to keyPath.
func generateSigningKey(idfPath, keyPath string) error {
	script := ". " + shq(filepath.Join(idfPath, "export.sh")) +
		" >/dev/null 2>&1 && espsecure.py generate_signing_key --version 2 " + shq(keyPath)
	outb, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("espsecure failed to generate the signing key: %w\n%s", err, string(outb))
	}
	return nil
}

// ensureDeploysGitignore makes sure the deploys directory never leaks a key into git.
func ensureDeploysGitignore(dir string) {
	gi := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gi); err == nil {
		return
	}
	_ = os.WriteFile(gi, []byte("# Signing keys are secrets -- never commit them.\n*.pem\n*.key\n!.gitignore\n"), 0o644)
}

// normalizeMAC lowercases a MAC and strips every non-hex character: "28:84:85:67:57:80" -> "288485675780".
func normalizeMAC(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// shq single-quotes a string for safe use inside a bash -c command line.
func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
