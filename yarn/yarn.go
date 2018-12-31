package yarn

import (
	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"os"
	"os/exec"
	"path/filepath"
)

const Dependency = "yarn"

type Yarn struct {
	Layer layers.DependencyLayer
}

func (y Yarn) InstallOffline(location string) error {
	y.setConfig("yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache"))
	y.setConfig("yarn-offline-mirror-pruning", "false")
	if err := y.install(location, true); err != nil {
		return err
	}
	return y.check(location, true)
}

func (y Yarn) InstallOnline(location string) error {
	y.setConfig("yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache"))
	y.setConfig("yarn-offline-mirror-pruning", "true")
	if err := y.install(location, false); err != nil {
		return err
	}
	return y.check(location, false)
}

func (y Yarn) setConfig(key, value string) error {
	return y.run("config", "set", key, value)
}

func (y Yarn) install(location string, offline bool) error {
	args := []string{
		"install",
		"--pure-lockfile",
		"--ignore-engines",
		"--cache-folder",
		filepath.Join(location, ".cache/yarn"),
		"--modules-folder",
		filepath.Join(location, "node_modules"),
	}

	if offline {
		args = append(args, "--offline")
	}

	return y.run(location, args...)
}

func (y Yarn) check(location string, offline bool) error {
	args := []string{"check"}

	if offline {
		args = append(args, "--offline")
	}

	return y.run(location, args...)
}

func (y Yarn) run(dir string, args ...string) error {
	cmd := exec.Command(filepath.Join(y.Layer.Root, "bin", "yarn"), args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Contributor struct {
	YarnLayer          layers.DependencyLayer
	buildContribution  bool
	launchContribution bool
}

func NewContributor(builder build.Build) (Contributor, bool, error) {
	plan, wantDependency := builder.BuildPlan[Dependency]
	if !wantDependency {
		return Contributor{}, false, nil
	}

	deps, err := builder.Buildpack.Dependencies()
	if err != nil {
		return Contributor{}, false, err
	}

	dep, err := deps.Best(Dependency, plan.Version, builder.Stack)
	if err != nil {
		return Contributor{}, false, err
	}

	contributor := Contributor{YarnLayer: builder.Layers.DependencyLayer(dep)}

	if _, ok := plan.Metadata["build"]; ok {
		contributor.buildContribution = true
	}

	if _, ok := plan.Metadata["launch"]; ok {
		contributor.launchContribution = true
	}

	return contributor, true, nil
}

func (n Contributor) Contribute() error {
	return n.YarnLayer.Contribute(func(artifact string, layer layers.DependencyLayer) error {
		layer.Logger.SubsequentLine("Expanding to %s", layer.Root)
		return helper.ExtractTarGz(artifact, layer.Root, 1)
	}, n.flags()...)
}

func (n Contributor) flags() []layers.Flag {
	flags := []layers.Flag{layers.Cache}

	if n.buildContribution {
		flags = append(flags, layers.Build)
	}

	if n.launchContribution {
		flags = append(flags, layers.Launch)
	}

	return flags
}
