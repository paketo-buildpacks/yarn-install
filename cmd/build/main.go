package main

import (
	"fmt"
	"github.com/cloudfoundry/libjavabuildpack"
	"github.com/cloudfoundry/yarn-cnb/build"
	"os"
)

func main() {
	builder, err := libjavabuildpack.DefaultBuild()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create default builder: %s", err)
		os.Exit(100)
	}

	if err := builder.Launch.WriteMetadata(build.CreateLaunchMetadata()); err != nil {
		builder.Logger.Info("failed to write launch.toml: %s", err)
		builder.Failure(100)
	}

	builder.Success()
}
