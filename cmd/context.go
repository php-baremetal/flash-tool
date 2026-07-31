package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
	phpDir    string
	idfPath   string
	dargs     []string
	fetches   []string
	port      string
	buildDir  string
	sdkconfig string
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
	m, err := manifest.LoadManifest(phpDir, repo.DefaultVersion)
	if err != nil {
		return nil, err
	}
	dargs, fetches, err := build.Args(cfg, m, repo.DefaultVersion)
	if err != nil {
		return nil, err
	}
	// For an `embedded` project, build the PHP source into the firmware image. The
	// project dir is the current working directory (where the config lives).
	if wd, err := os.Getwd(); err == nil {
		if arg, ok := build.EmbedArg(cfg, wd); ok {
			dargs = append(dargs, arg)
		}
	}
	return &buildContext{
		phpDir:    phpDir,
		idfPath:   idfPath,
		dargs:     dargs,
		fetches:   fetches,
		port:      cfg.Board.Port,
		buildDir:  projectBuildDir(),
		sdkconfig: projectSdkconfig(),
	}, nil
}
