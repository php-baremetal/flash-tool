package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"phpflash/internal/build"
	"phpflash/internal/config"
	"phpflash/internal/manifest"
	"phpflash/internal/platform"
)

// resolvePort picks the serial port: the -p flag, else the config's [board].port, else the
// first serial device that actually exists (/dev/ttyACM* first -- the Pico's CH343P bridge),
// else "" so ESP-IDF autodetects. Detection only checks that the device node exists; it never
// opens a port, so it can't hang on or disturb a device that isn't the board.
func resolvePort(flag, cfgPort string) string {
	if flag != "" {
		return flag
	}
	if cfgPort != "" {
		return cfgPort
	}
	return platform.DetectPort()
}

// projectBuildDir / projectSdkconfig locate the project's own build output and
// sdkconfig, so each project keeps an isolated build in its own folder.
func projectBuildDir() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "build")
}

func projectSdkconfig() string {
	wd, _ := os.Getwd()
	return filepath.Join(wd, "build", "sdkconfig")
}

// resolveDirs applies the flag>config>env>default precedence to the ESP-IDF and
// php-esp32 paths.
func resolveDirs(idfFlag, phpFlag string, cfg *config.Config) (idfPath, phpDir string) {
	var cIdf, cPhp string
	if cfg != nil {
		cIdf, cPhp = cfg.EspIdf.Path, cfg.PhpEsp32.Path
	}
	idfPath = config.Resolve(config.ResolveInputs{Flag: idfFlag, Config: cIdf, Env: os.Getenv("IDF_PATH"), Default: config.DefaultIdfPath()}).Value
	phpDir = config.Resolve(config.ResolveInputs{Flag: phpFlag, Config: cPhp, Env: os.Getenv("PHP_ESP32_DIR"), Default: config.DefaultPhpEsp32Path()}).Value
	return
}

// buildContext holds everything build/flash need. The Invoker is constructed by each
// command so it can choose the output sink (a progress bar for build, raw for flash).
type buildContext struct {
	phpDir      string
	idfPath     string
	dargs       []string
	fetches     []string
	port        string
	buildDir    string
	sdkconfig   string
	storageType string
	board       string // the configured board key, e.g. "esp32-s3-eth"
	idfTarget   string // the board family's ESP-IDF target, e.g. "esp32s3" ("" if unresolved)
}

func loadBuildContext(idfFlag, phpFlag string) (*buildContext, error) {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return nil, fmt.Errorf("no %s in this directory (run `phpflash init` first): %w", config.FileName, err)
	}
	idfPath, phpDir := resolveDirs(idfFlag, phpFlag, cfg)

	repo, err := manifest.LoadRepo(phpDir)
	if err != nil {
		return nil, fmt.Errorf("php-esp32 not found at %s (run `phpflash system-setup`): %w", phpDir, err)
	}
	// The PHP language version: the project's [php] version if it pins one, else the repo default.
	phpVersion := cfg.Php.Version
	if phpVersion == "" {
		phpVersion = repo.DefaultVersion
	}
	m, err := manifest.LoadManifest(phpDir, phpVersion)
	if err != nil {
		if avail, e := manifest.AvailableVersions(phpDir); e == nil && len(avail) > 0 {
			return nil, fmt.Errorf("PHP version %q is not installed in %s (available: %s)",
				phpVersion, phpDir, strings.Join(avail, ", "))
		}
		return nil, err
	}
	dargs, fetches, err := build.Args(cfg, m, phpVersion)
	if err != nil {
		return nil, err
	}
	// Pin the ESP-IDF target from the board's family. Without it, idf.py "guesses" the target from
	// any stray in-source sdkconfig -- which silently builds the wrong architecture when that file is
	// left over from a different family (e.g. an esp32p4 sdkconfig while building an esp32s3 board).
	idfTarget, _ := manifest.TargetForBoard(phpDir, cfg.Board.Target)
	if idfTarget != "" {
		dargs = append(dargs, "-DIDF_TARGET="+idfTarget)
	}
	// For an `embedded` project, build the PHP source into the firmware image. The
	// project dir is the current working directory (where the config lives).
	if wd, err := os.Getwd(); err == nil {
		if arg, ok := build.EmbedArg(cfg, wd); ok {
			dargs = append(dargs, arg)
		}
		// A framework with a nested front controller (Laravel: public/index.php) sets [php] entry.
		if arg, ok := build.EntryArg(cfg); ok {
			dargs = append(dargs, arg)
		}
		// Custom C extensions the project ships in ./firmware/exts are compiled into the firmware.
		if arg, ok := build.ProjectExtsArg(wd); ok {
			dargs = append(dargs, arg)
		}
		// A full-openssl project needs an openssl.cnf shipped with its source; create it.
		if err := build.EnsureOpenSSLConf(cfg, wd); err != nil {
			return nil, fmt.Errorf("openssl.cnf: %w", err)
		}
		// If it set a custom config_path, tell the firmware where to read it.
		if arg, ok := build.OpenSSLConfArg(cfg); ok {
			dargs = append(dargs, arg)
		}
		// A TLS-client project ships the host's root CAs so the device can verify HTTPS peers.
		if src, err := build.EnsureTLSCerts(cfg, wd); err != nil {
			return nil, fmt.Errorf("CA bundle: %w", err)
		} else if src != "" {
			fmt.Fprintf(os.Stderr, "==> copied root CAs from %s\n", src)
		}
		if arg, ok := build.TLSCAArg(cfg); ok {
			dargs = append(dargs, arg)
		}
		// Static DNS servers ([network] dns), if any.
		if arg, ok := build.DNSArg(cfg); ok {
			dargs = append(dargs, arg)
		}
	}
	return &buildContext{
		phpDir:      phpDir,
		idfPath:     idfPath,
		dargs:       dargs,
		fetches:     fetches,
		port:        cfg.Board.Port,
		buildDir:    projectBuildDir(),
		sdkconfig:   projectSdkconfig(),
		storageType: cfg.StorageType,
		board:       cfg.Board.Target,
		idfTarget:   idfTarget,
	}, nil
}
