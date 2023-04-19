package yarninstall

import (
	"path/filepath"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/fs"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface SBOMGenerator --output fakes/sbom_generator.go
type SBOMGenerator interface {
	Generate(dir string) (sbom.SBOM, error)
}

//go:generate faux --interface BuildProcess --output fakes/build_process.go
type BuildProcess interface {
	Build(context packit.BuildContext, installProcess InstallProcess, sbomGenerator SBOMGenerator, symlinker SymlinkManager, entryResolver EntryResolver, projectPath, tempDir string) (packit.BuildResult, error)
}

//go:generate faux --interface InstallProcess --output fakes/install_process.go
type InstallProcess interface {
	ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error)
	SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath string) (string, error)
	Execute(workingDir, modulesLayerPath string, launch bool) error
}

//go:generate faux --interface EntryResolver --output fakes/entry_resolver.go
type EntryResolver interface {
	MergeLayerTypes(string, []packit.BuildpackPlanEntry) (launch, build bool)
}

//go:generate faux --interface SymlinkManager --output fakes/symlink_manager.go
type SymlinkManager interface {
	Link(oldname, newname string) error
	Unlink(path string) error
}

func Build(pathParser PathParser,
	yarnrcYmlParser YarnrcYmlParser,
	berry BuildProcess,
	classic BuildProcess,
	berryInstall InstallProcess,
	classicInstall InstallProcess,
	sbomGenerator SBOMGenerator,
	entryResolver EntryResolver,
	logger scribe.Emitter,
	symlinker SymlinkManager,
	tmpDir string) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		projectPath, err := pathParser.Get(context.WorkingDir)
		if err != nil {
			return packit.BuildResult{}, err
		}

		exists, err := fs.Exists(filepath.Join(projectPath, ".yarnrc.yml"))
		if err != nil {
			panic(err)
		}

		if exists {
			return berry.Build(context, berryInstall, sbomGenerator, symlinker, entryResolver, projectPath, tmpDir)
		}

		return classic.Build(context, classicInstall, sbomGenerator, symlinker, entryResolver, projectPath, tmpDir)
	}
}
