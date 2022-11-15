package berry

import (
	"github.com/paketo-buildpacks/packit/v2"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

type BerryBuild struct{}

func NewBerryBuild() BerryBuild {
	return BerryBuild{}
}

func (bb BerryBuild) Build(ctx packit.BuildContext,
	installProcess yarninstall.InstallProcess,
	sbomGenerator yarninstall.SBOMGenerator,
	symlinker yarninstall.SymlinkManager,
	entryResolver yarninstall.EntryResolver,
	projectPath, tmpDir string) (packit.BuildResult, error) {
	return packit.BuildResult{}, nil
}
