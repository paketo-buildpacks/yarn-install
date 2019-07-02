package yarn

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/buildpack/libbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/pkg/errors"
)

const (
	offlineCacheDir = "npm-packages-offline-cache"
	unmetDepWarning = "Unmet dependencies don't fail yarn install but may cause runtime issues\nSee: https://github.com/npm/npm/issues/7494"
)

type Runner interface {
	Run(bin, dir string, args ...string) error
	RunWithOutput(bin, dir string, quiet bool, args ...string) (string, error)
}

type CLI struct {
	appDir  string
	binary  string
	offline bool
	runner  Runner
	log     logger.Logger
}

func NewCLI(appDir, binary string, runner Runner, logger logger.Logger) (CLI, error) {
	offline, err := helper.FileExists(filepath.Join(appDir, offlineCacheDir))
	if err != nil {
		return CLI{}, err
	}

	return CLI{
		appDir:  appDir,
		binary:  binary,
		offline: offline,
		runner:  runner,
		log:     logger,
	}, nil
}

func (c CLI) Install(modulesDir, cacheDir string) error {

	nodePath := os.Getenv("NODE_HOME")
	c.log.Info(fmt.Sprintf("NODE HOME VALUE %s\n", nodePath))
	if err := os.Setenv("npm_config_nodedir", nodePath); err != nil {
		return errors.Wrap(err, "error setting up rebuild config")
	}

	args := []string{
		"install",
		"--pure-lockfile",
		"--ignore-engines",
		"--cache-folder",
		cacheDir,
	}

	mode := "online"

	if c.offline {
		mode = "offline"
		args = append(args, "--offline")

		offlineCache := filepath.Join(c.appDir, offlineCacheDir)
		if err := c.SetConfig(c.appDir, "yarn-offline-mirror", offlineCache); err != nil {
			return err
		}

		if err := c.SetConfig(c.appDir, "yarn-offline-mirror-pruning", "false"); err != nil {
			return err
		}
	}

	c.log.Info("Running yarn in %s mode", mode)

	output, err := c.runner.RunWithOutput(c.binary, modulesDir, false, args...)
	if err != nil {
		return err
	}

	c.warnUnmetDependencies(output)

	return nil
}

func (c CLI) Check(appDir string) error {
	args := []string{"check"}

	if c.offline {
		args = append(args, "--offline")
	}

	if out, err := c.runner.RunWithOutput(c.binary, appDir, true, args...); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return err
		}
		c.log.Info(out)
		c.log.Info("yarn.lock is outdated")
	} else {
		c.log.Info("yarn.lock and package.json match")
	}
	return nil
}

func (c CLI) SetConfig(location, key, value string) error {
	return c.runner.Run(c.binary, location, "config", "set", key, value)
}

func (c CLI) warnUnmetDependencies(installLog string) {
	installLog = strings.ToLower(installLog)
	unmet := strings.Contains(installLog, "unmet dependency") || strings.Contains(installLog, "unmet peer dependency")
	if unmet {
		c.log.Info(unmetDepWarning)
	}
}
