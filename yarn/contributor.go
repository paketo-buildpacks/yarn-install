package yarn

import (
	"fmt"
	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/buildpack"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"os"
)

const (
	Dependency = "yarn"
)

type Contributor struct {
	context            build.Build
	YarnLayer          layers.DependencyLayer
	buildContribution  bool
	launchContribution bool
}

func NewContributor(context build.Build) (Contributor, bool, error) {
	plan, wantDependency := context.BuildPlan[Dependency]
	if !wantDependency {
		return Contributor{}, false, nil
	}

	version := plan.Version
	if version == "" {
		var err error
		if version, err = context.Buildpack.DefaultVersion(Dependency); err != nil {
			return Contributor{}, false, err
		}
	}

	deps, err := context.Buildpack.Dependencies()
	if err != nil {
		return Contributor{}, false, errors.Wrap(err, "failed to get dependencies")
	}


	var dep buildpack.Dependency
	if entry, ok := plan.Metadata["override"]; ok {
		// cast entry as string
		if stringEntry, ok := entry.(string); ok {
			var overrideDep buildpack.Dependency

			if err := yaml.Unmarshal([]byte(stringEntry), &overrideDep); err != nil {
				return Contributor{}, false, err
			}

			dep = overrideDep
		}
	} else {
		dep, err = deps.Best(Dependency, version, context.Stack)
		if err != nil {
			return Contributor{}, false, err
		}
	}

	contributor := Contributor{
		context: context,
		YarnLayer: context.Layers.DependencyLayer(dep),
	}

	if _, ok := plan.Metadata["build"]; ok {
		contributor.buildContribution = true
	}

	if _, ok := plan.Metadata["launch"]; ok {
		contributor.launchContribution = true
	}

	return contributor, true, nil
}

func (c *Contributor) Contribute() error {
	return c.YarnLayer.Contribute(func(artifact string, layer layers.DependencyLayer) error {
		nodeHome := os.Getenv("NODE_HOME")
		layer.Logger.SubsequentLine(fmt.Sprintf("NODE_HOME Value %s",nodeHome))
		layer.Logger.SubsequentLine("Expanding to %s", layer.Root)

		return helper.ExtractTarGz(artifact, layer.Root, 1)
	}, c.flags()...)
}

func (c *Contributor) flags() []layers.Flag {
	flags := []layers.Flag{layers.Cache}

	if c.buildContribution {
		flags = append(flags, layers.Build)
	}

	if c.launchContribution {
		flags = append(flags, layers.Launch)
	}

	return flags
}
