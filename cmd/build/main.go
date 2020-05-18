package main

import (
	"os"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/cargo"
	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/paketo-buildpacks/packit/scribe"
	"github.com/paketo-buildpacks/yarn-install/yarn"
)

func main() {
	logger := scribe.NewLogger(os.Stdout)

	transport := cargo.NewTransport()
	executable := pexec.NewExecutable("yarn")
	summer := fs.NewChecksumCalculator()
	installProcess := yarn.NewYarnInstallProcess(executable, summer, logger)
	dependencyService := postal.NewService(transport)

	clock := yarn.NewClock(time.Now)
	cacheHandler := yarn.NewCacheHandler()

	packit.Build(yarn.Build(dependencyService, cacheHandler, installProcess, clock, logger))
}
