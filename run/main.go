package main

import (
	"os"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/cargo"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/paketo-buildpacks/packit/scribe"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

func main() {
	packageJSONParser := yarninstall.NewPackageJSONParser()
	logger := scribe.NewLogger(os.Stdout)
	transport := cargo.NewTransport()
	executable := pexec.NewExecutable("yarn")
	summer := fs.NewChecksumCalculator()
	installProcess := yarninstall.NewYarnInstallProcess(executable, summer, logger)
	dependencyService := postal.NewService(transport)
	cacheHandler := yarninstall.NewCacheHandler()
	projectPathParser := yarninstall.NewProjectPathParser()

	packit.Run(
		yarninstall.Detect(
			projectPathParser,
			packageJSONParser,
		),
		yarninstall.Build(
			projectPathParser,
			dependencyService,
			cacheHandler,
			installProcess,
			chronos.DefaultClock,
			logger,
		),
	)
}
