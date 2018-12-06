package main

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/yarn-cnb/modules"
	"github.com/cloudfoundry/yarn-cnb/yarn"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/build"
)

func main() {
	context, err := build.DefaultBuild()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create a default build context: %s", err)
		os.Exit(100)
	}

	code, err := runBuild(context)
	if err != nil {
		context.Logger.Info(err.Error())
	}

	os.Exit(code)
}

func runBuild(context build.Build) (int, error) {
	context.Logger.FirstLine(context.Logger.PrettyIdentity(context.Buildpack))

	yarnContributor, willContributeYarn, err := yarn.NewContributor(context)
	if err != nil {
		return context.Failure(102), err
	}

	if willContributeYarn {
		if err := yarnContributor.Contribute(); err != nil {
			return context.Failure(103), err
		}
	}

	modulesContributor, willContributeModules, err := modules.NewContributor(context, yarn.Yarn{})
	if err != nil {
		return context.Failure(104), err
	}

	if willContributeModules {
		if err := modulesContributor.Contribute(); err != nil {
			return context.Failure(105), err
		}
	}

	return context.Success(buildplan.BuildPlan{})
}
