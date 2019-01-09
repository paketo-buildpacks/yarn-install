package yarn

import (
	"os"
	"path/filepath"

	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/layers"
)

const (
	Dependency = "yarn"
	CacheDir   = "yarn-cache"
	ModulesDir = "node_modules"
)

type Runner interface {
	Run(bin, dir string, args ...string) error
}

type Logger interface {
	Info(format string, args ...interface{})
}

type Yarn struct {
	Runner Runner
	Logger Logger
	Layer  layers.Layer
}

func (y Yarn) InstallOffline(location string) error {
	if err := y.setConfig(location, "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache")); err != nil {
		return err
	}

	if err := y.setConfig(location, "yarn-offline-mirror-pruning", "false"); err != nil {
		return err
	}

	if err := y.install(location, true); err != nil {
		return err
	}

	return y.check(location, true)
}

func (y Yarn) InstallOnline(location string) error {
	if err := y.setConfig(location, "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache")); err != nil {
		return err
	}

	if err := y.setConfig(location, "yarn-offline-mirror-pruning", "true"); err != nil {
		return err
	}

	if err := y.install(location, false); err != nil {
		return err
	}

	return y.check(location, false)
}

func (y Yarn) setConfig(location, key, value string) error {
	return y.Runner.Run(filepath.Join(y.Layer.Root, "bin", "yarn"), location, "config", "set", key, value)
}

func (y Yarn) moveDir(name, location string) error {
	dir := filepath.Join(y.Layer.Root, name)

	if exists, err := helper.FileExists(dir); err != nil {
		return err
	} else if !exists {
		return nil
	}

	y.Logger.Info("Reusing existing %s", name)
	if err := helper.CopyDirectory(dir, filepath.Join(location, name)); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (y Yarn) install(location string, offline bool) error {
	if err := y.moveDir(ModulesDir, location); err != nil {
		return err
	}

	if err := y.moveDir(CacheDir, location); err != nil {
		return err
	}

	args := []string{
		"install",
		"--pure-lockfile",
		"--ignore-engines",
		"--cache-folder",
		filepath.Join(location, CacheDir),
		"--modules-folder",
		filepath.Join(location, ModulesDir),
	}

	if offline {
		args = append(args, "--offline")
	}

	return y.Runner.Run(filepath.Join(y.Layer.Root, "bin", "yarn"), location, args...)
}

func (y Yarn) check(location string, offline bool) error {
	args := []string{"check"}

	if offline {
		args = append(args, "--offline")
	}

	return y.Runner.Run(filepath.Join(y.Layer.Root, "bin", "yarn"), location, args...)
}
