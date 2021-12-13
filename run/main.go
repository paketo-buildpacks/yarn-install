package main

import (
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

type SBOMGenerator struct{}

func (s SBOMGenerator) Generate(path string) (sbom.SBOM, error) {
	return sbom.Generate(path)
}

func main() {
	packageJSONParser := yarninstall.NewPackageJSONParser()
	logger := scribe.NewLogger(os.Stdout)
	executable := pexec.NewExecutable("yarn")
	summer := fs.NewChecksumCalculator()
	installProcess := yarninstall.NewYarnInstallProcess(executable, summer, logger)
	projectPathParser := yarninstall.NewProjectPathParser()
	sbomGenerator := SBOMGenerator{}

	packit.Run(
		yarninstall.Detect(
			projectPathParser,
			packageJSONParser,
		),
		yarninstall.Build(projectPathParser, installProcess, chronos.DefaultClock, logger, sbomGenerator),
	)
}
