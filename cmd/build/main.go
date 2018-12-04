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
	builder, err := build.DefaultBuild()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create default builder: %s", err)
		os.Exit(100)
	}

	code, err := runBuild(builder)
	if err != nil {
		builder.Logger.Info(err.Error())
	}

	os.Exit(code)
}

func runBuild(builder build.Build) (int, error) {
	builder.Logger.FirstLine(builder.Logger.PrettyIdentity(builder.Buildpack))

	yarnContributor, willContributeYarn, err := yarn.NewContributor(builder)
	if err != nil {
		return builder.Failure(102), err
	}

	if willContributeYarn {
		if err := yarnContributor.Contribute(); err != nil {
			return builder.Failure(103), err
		}
	}

	modulesContributor, willContributeModules, err := modules.NewContributor(builder, yarn.Yarn{})
	if err != nil {
		return builder.Failure(104), err
	}

	if willContributeModules {
		if err := modulesContributor.Contribute(); err != nil {
			return builder.Failure(105), err
		}
	}

	return builder.Success(buildplan.BuildPlan{})
}
