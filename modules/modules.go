package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/buildpack/libbuildpack/application"
	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/layers"
)

const Dependency = "modules"

type PackageManager interface {
	InstallOffline(location string) error
	InstallOnline(location string) error
}

type Metadata struct {
	Hash string
}

func (m Metadata) Identity() (name string, version string) {
	return Dependency, m.Hash
}

type Contributor struct {
	Metadata           Metadata
	buildContribution  bool
	launchContribution bool
	pkgManager         PackageManager
	app                application.Application
	modulesLayer       layers.Layer
	launchLayer        layers.Layers
}

func NewContributor(context build.Build, pkgManager PackageManager) (Contributor, bool, error) {
	plan, shouldUseNPM := context.BuildPlan[Dependency]
	if !shouldUseNPM {
		return Contributor{}, false, nil
	}

	lockFile := filepath.Join(context.Application.Root, "yarn.lock")
	if exists, err := helper.FileExists(lockFile); err != nil {
		return Contributor{}, false, err
	} else if !exists {
		return Contributor{}, false, fmt.Errorf(`unable to find "yarn.lock"`)
	}

	buf, err := ioutil.ReadFile(lockFile)
	if err != nil {
		return Contributor{}, false, err
	}

	hash := sha256.Sum256(buf)

	contributor := Contributor{
		app:          context.Application,
		pkgManager:   pkgManager,
		modulesLayer: context.Layers.Layer(Dependency),
		launchLayer:  context.Layers,
		Metadata:     Metadata{hex.EncodeToString(hash[:])},
	}

	if _, ok := plan.Metadata["build"]; ok {
		contributor.buildContribution = true
	}

	if _, ok := plan.Metadata["launch"]; ok {
		contributor.launchContribution = true
	}

	return contributor, true, nil
}

func (c Contributor) Contribute() error {
	return c.modulesLayer.Contribute(c.Metadata, func(layer layers.Layer) error {
		offlineCache := filepath.Join(c.app.Root, "npm-packages-offline-cache")

		offline, err := helper.FileExists(offlineCache)
		if err != nil {
			return fmt.Errorf("unable to stat node_modules: %s", err.Error())
		}

		if offline {
			c.modulesLayer.Logger.Info("Running yarn in offline mode")
			if err := c.pkgManager.InstallOffline(c.app.Root); err != nil {
				return fmt.Errorf("unable to install node_modules: %s", err.Error())
			}
		} else {
			c.modulesLayer.Logger.Info("Running yarn in online mode")
			if err := c.pkgManager.InstallOnline(c.app.Root); err != nil {
				return fmt.Errorf("unable to install node_modules: %s", err.Error())
			}
		}

		nodeModules := filepath.Join(c.app.Root, "node_modules")

		if err := os.MkdirAll(layer.Root, 0777); err != nil {
			return fmt.Errorf("unable make layer: %s", err.Error())
		}

		if err := helper.CopyDirectory(nodeModules, layer.Root); err != nil {
			return fmt.Errorf(`unable to copy "%s" to "%s": %s`, nodeModules, layer.Root, err.Error())
		}

		if err := os.RemoveAll(nodeModules); err != nil {
			return fmt.Errorf("unable to remove node_modules from the app dir: %s", err.Error())
		}

		if err := layer.OverrideSharedEnv("NODE_PATH", layer.Root); err != nil {
			return err
		}

		if err := layer.OverrideSharedEnv("npm_config_nodedir", os.Getenv("NODE_HOME")); err != nil {
			return err
		}

		return c.launchLayer.WriteMetadata(layers.Metadata{
			Processes: []layers.Process{{"web", "yarn start"}},
		})
	}, c.flags()...)
}

func (c Contributor) flags() []layers.Flag {
	flags := []layers.Flag{layers.Cache}

	if c.buildContribution {
		flags = append(flags, layers.Build)
	}

	if c.launchContribution {
		flags = append(flags, layers.Launch)
	}

	return flags
}
