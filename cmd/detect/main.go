package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/yarn-cnb/modules"
	"github.com/cloudfoundry/yarn-cnb/node"
	"github.com/cloudfoundry/yarn-cnb/yarn"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/detect"
	"github.com/cloudfoundry/libcfbuildpack/layers"
)

func main() {
	context, err := detect.DefaultDetect()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create a default detection context: %s", err)
		os.Exit(100)
	}

	code, err := runDetect(context)
	if err != nil {
		context.Logger.Info(err.Error())
	}

	os.Exit(code)
}

func runDetect(context detect.Detect) (int, error) {
	yarnLock := filepath.Join(context.Application.Root, "yarn.lock")
	if exists, _ := layers.FileExists(yarnLock); !exists {
		return context.Fail(), fmt.Errorf(`no "yarn.lock" found at: %s`, yarnLock)
	}

	packageJSON := filepath.Join(context.Application.Root, "package.json")
	if exists, _ := layers.FileExists(packageJSON); !exists {
		return context.Fail(), fmt.Errorf(`no "package.json" found at: %s`, packageJSON)
	}

	version, err := node.GetNodeVersion(packageJSON)
	if err != nil {
		return context.Fail(), fmt.Errorf(`unable to parse "package.json": %s`, err.Error())
	}

	return context.Pass(buildplan.BuildPlan{
		node.Dependency: buildplan.Dependency{
			Version:  version,
			Metadata: buildplan.Metadata{"build": true, "launch": true},
		},
		yarn.Dependency: buildplan.Dependency{
			Metadata: buildplan.Metadata{"build": true, "launch": true},
		},
		modules.Dependency: buildplan.Dependency{
			Metadata: buildplan.Metadata{"launch": true},
		},
	})
}
