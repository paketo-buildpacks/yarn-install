package main

import (
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/yarn-install/berry"
	"github.com/paketo-buildpacks/yarn-install/classic"
	"github.com/paketo-buildpacks/yarn-install/common"

	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

type SBOMGenerator struct{}

func (s SBOMGenerator) Generate(path string) (sbom.SBOM, error) {
	return sbom.Generate(path)
}

func main() {
	packageJSONParser := common.NewPackageJSONParser()
	yarnrcYmlParser := common.NewYarnrcYmlParser()
	logger := scribe.NewEmitter(os.Stdout).WithLevel(os.Getenv("BP_LOG_LEVEL"))
	berryInstallProcess := berry.NewBerryInstallProcess(pexec.NewExecutable("yarn"), fs.NewChecksumCalculator(), logger)
	classicInstallProcess := classic.NewYarnInstallProcess(pexec.NewExecutable("yarn"), fs.NewChecksumCalculator(), logger)
	classic := classic.NewClassicBuild(logger)
	berry := berry.NewBerryBuild(logger)
	projectPathParser := common.NewProjectPathParser()
	symlinker := common.NewSymlinker()
	sbomGenerator := SBOMGenerator{}
	entryResolver := draft.NewPlanner()
	tmpDir := os.TempDir()

	packit.Run(
		yarninstall.Detect(
			projectPathParser,
			packageJSONParser,
			yarnrcYmlParser,
		),
		yarninstall.Build(projectPathParser,
			yarnrcYmlParser,
			berry,
			classic,
			berryInstallProcess,
			classicInstallProcess,
			sbomGenerator,
			entryResolver,
			logger,
			symlinker,
			tmpDir,
		),
	)
}
