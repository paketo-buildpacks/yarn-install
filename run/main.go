package main

import (
	"log"
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/pexec"
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
	logger := scribe.NewLogger(os.Stdout)
	executable := pexec.NewExecutable("yarn")
	summer := fs.NewChecksumCalculator()
	installProcess := yarninstall.NewYarnInstallProcess(executable, summer, logger)
	projectPathParser := yarninstall.NewProjectPathParser()
	sbomGenerator := SBOMGenerator{}
	bindingResolver := servicebindings.NewResolver()
	symlinker := yarninstall.NewSymlinker()
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
			bindingResolver,
			home,
			symlinker,
			installProcess,
			sbomGenerator,
			chronos.DefaultClock,
			logger),
	)
}
