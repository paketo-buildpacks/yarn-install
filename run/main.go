package main

import (
	"log"
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/packit/v2/servicebindings"

	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

type SBOMGenerator struct{}

func (s SBOMGenerator) Generate(path string) (sbom.SBOM, error) {
	return sbom.Generate(path)
}

func main() {
	packageJSONParser := yarninstall.NewPackageJSONParser()
	emitter := scribe.NewEmitter(os.Stdout).WithLevel(os.Getenv("BP_LOG_LEVEL"))
	// installProcess := yarninstall.NewYarnInstallProcess(pexec.NewExecutable("yarn"), fs.NewChecksumCalculator(), scribe.NewLogger(os.Stdout))
	projectPathParser := yarninstall.NewProjectPathParser()
	// sbomGenerator := SBOMGenerator{}
	symlinker := yarninstall.NewSymlinker()
	packageManagerConfigurationManager := yarninstall.NewPackageManagerConfigurationManager(servicebindings.NewResolver(), emitter)
	entryResolver := draft.NewPlanner()
	home, err := os.UserHomeDir()
	if err != nil {
		// not tested
		log.Fatal(err)
	}

	packit.Run(
		yarninstall.Detect(
			projectPathParser,
			packageJSONParser,
		),
		yarninstall.Build(projectPathParser,
			entryResolver,
			packageManagerConfigurationManager,
			home,
			symlinker,
			nil,
			nil,
			emitter),
	)
}
