package main

import (
	"os"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
	"github.com/paketo-buildpacks/packit/servicebindings"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

func main() {
	packageJSONParser := yarninstall.NewPackageJSONParser()
	logger := scribe.NewLogger(os.Stdout)
	executable := pexec.NewExecutable("yarn")
	summer := fs.NewChecksumCalculator()
	installProcess := yarninstall.NewYarnInstallProcess(executable, summer, logger)
	projectPathParser := yarninstall.NewProjectPathParser()
	bindingResolver := servicebindings.NewResolver()
	symlinker := yarninstall.NewSymlinker()

	packit.Run(
		yarninstall.Detect(
			projectPathParser,
			packageJSONParser,
		),
		yarninstall.Build(projectPathParser,
			bindingResolver,
			symlinker,
			installProcess,
			chronos.DefaultClock,
			logger),
	)
}
