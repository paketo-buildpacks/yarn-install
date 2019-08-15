package main

import (
	"fmt"
	"github.com/buildpack/libbuildpack/logger"
	"github.com/cloudfoundry/yarn-cnb/modules"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/yarn-cnb/utils"

	"github.com/cloudfoundry/yarn-cnb/yarn"

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

	pkgManager, err := yarn.NewCLI(
		context.Application.Root,
		filepath.Join(yarnContributor.YarnLayer.Layer.Root, "bin", "yarn"),
		utils.CommandRunner{},
		logger.NewLogger(os.Stderr, os.Stdout),
	)

	modulesContributor, willContributeModules, err := modules.NewContributor(context, pkgManager)
	if err != nil {
		return context.Failure(104), err
	}

	if willContributeModules {
		if err := modulesContributor.Contribute(); err != nil {
			return context.Failure(105), err
		}
	}

	return context.Success()
}
