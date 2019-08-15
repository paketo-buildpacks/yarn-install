package yarn

import (
	"fmt"
	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/pkg/errors"
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
	plan, wantDependency, err := context.Plans.GetShallowMerged(Dependency)
	if err != nil {
		return Contributor{}, false, err
	}

	if !wantDependency {
		return Contributor{}, false, nil
	}

	contributor := Contributor{context: context}
	contributor.buildContribution, _ = plan.Metadata["build"].(bool)
	contributor.launchContribution, _ = plan.Metadata["launch"].(bool)

	return contributor, true, nil
}

func (c *Contributor) Contribute() error {
	deps, err := c.context.Buildpack.Dependencies()
	if err != nil {
		return errors.Wrap(err, "failed to get dependencies")
	}

	dep, err := deps.Best(Dependency, "*", c.context.Stack)
	if err != nil {
		return err
	}

	c.YarnLayer = c.context.Layers.DependencyLayer(dep)

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
