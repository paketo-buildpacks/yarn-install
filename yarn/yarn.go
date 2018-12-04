package yarn

import (
	"os"
	"os/exec"

	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/layers"
)

const Dependency = "yarn"

type Yarn struct{}

func (n Yarn) Install(location string) error {
	return n.run(location, "install")
}

func (n Yarn) Rebuild(location string) error {
	return n.run(location, "rebuild")
}

func (n Yarn) run(dir string, args ...string) error {
	cmd := exec.Command("yarn", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Contributor struct {
	buildContribution  bool
	launchContribution bool
	layer              layers.DependencyLayer
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

	contributor := Contributor{layer: builder.Layers.DependencyLayer(dep)}

	if _, ok := plan.Metadata["build"]; ok {
		contributor.buildContribution = true
	}

	if _, ok := plan.Metadata["launch"]; ok {
		contributor.launchContribution = true
	}

	return contributor, true, nil
}

func (n Contributor) Contribute() error {
	return n.layer.Contribute(func(artifact string, layer layers.DependencyLayer) error {
		layer.Logger.SubsequentLine("Expanding to %s", layer.Root)
		return layers.ExtractTarGz(artifact, layer.Root, 1)
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
