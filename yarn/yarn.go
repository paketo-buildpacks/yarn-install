package yarn

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/layers"
)

const (
	Dependency = "yarn"
	CacheDir   = "yarn-cache"
	ModulesDir = "node_modules"
	UNMET_DEP_WARNING = "Unmet dependencies don't fail yarn install but may cause runtime issues\nSee: https://github.com/npm/npm/issues/7494"
)

type Runner interface {
	Run(bin, dir string, args ...string) error
	RunWithOutput(bin, dir string, quiet bool, args ...string) (string, error)
}

type Logger interface {
	Info(format string, args ...interface{})
	Warning(format string, args ...interface{})
}

type Yarn struct {
	Runner Runner
	Logger Logger
	Layer  layers.Layer
}

func (y Yarn) InstallOffline(location, destination string) error {
	if err := y.setConfig(location, "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache")); err != nil {
		return err
	}

	if err := y.setConfig(location, "yarn-offline-mirror-pruning", "false"); err != nil {
		return err
	}

	if err := y.install(location, destination , true); err != nil {
		return err
	}

	return y.check(location, true)
}

func (y Yarn) InstallOnline(location, destination string) error {
	if err := y.setConfig(location, "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache")); err != nil {
		return err
	}

	if err := y.setConfig(location, "yarn-offline-mirror-pruning", "true"); err != nil {
		return err
	}

	if err := y.install(location, destination, false); err != nil {
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

func (y Yarn) install(location, destination string, offline bool) error {
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
		destination,
	}

	if offline {
		args = append(args, "--offline")
	}

	if output, err := y.Runner.RunWithOutput(filepath.Join(y.Layer.Root, "bin", "yarn"), location, false, args...); err != nil {
		return err
	} else {
		y.warnUnmetDependencies(output)
	}
	return nil
}

func (y Yarn) check(location string, offline bool) error {
	args := []string{"check"}

	if offline {
		args = append(args, "--offline")
	}
	if err := y.Runner.Run(filepath.Join(y.Layer.Root, "bin", "yarn"), location, args...); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return err
		}
		y.Logger.Warning("yarn.lock is outdated")
	} else {
		y.Logger.Info("yarn.lock and package.json match")
	}
	return nil
}

func (y Yarn) warnUnmetDependencies(installLog string) {
	installLog = strings.ToLower(installLog)
	unmet := strings.Contains(installLog, "unmet dependency") || strings.Contains(installLog, "unmet peer dependency")
	if unmet {
		y.Logger.Info(UNMET_DEP_WARNING)
	}
}

