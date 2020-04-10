package main

import (
	"os"
	"time"

	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/packit/cargo"
	"github.com/cloudfoundry/packit/fs"
	"github.com/cloudfoundry/packit/pexec"
	"github.com/cloudfoundry/packit/postal"
	"github.com/cloudfoundry/packit/scribe"
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
