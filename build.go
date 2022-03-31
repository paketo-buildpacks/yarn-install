package yarninstall

import (
	"path/filepath"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface SymlinkManager --output fakes/symlink_manager.go
type SymlinkManager interface {
	Link(oldname, newname string) error
	Unlink(path string) error
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

//go:generate faux --interface SBOMGenerator --output fakes/sbom_generator.go
type SBOMGenerator interface {
	Generate(dir string) (sbom.SBOM, error)
}

//go:generate faux --interface ConfigurationManager --output fakes/configuration_manager.go
type ConfigurationManager interface {
	DeterminePath(typ, platformDir, entry string) (path string, err error)
}

//go:generate faux --interface LayerBuilder --output fakes/layer_builder.go
type LayerBuilder interface {
	Build(context packit.BuildContext, currentModulesLayerPath, projectPath string) (packit.Layer, error)
}

func Build(pathParser PathParser,
	entryResolver EntryResolver,
	configurationManager ConfigurationManager,
	homeDir string,
	symlinker SymlinkManager,
	buildLayerBuilder LayerBuilder,
	launchLayerBuilder LayerBuilder,
	logger scribe.Emitter) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		projectPath, err := pathParser.Get(context.WorkingDir)
		if err != nil {
			return packit.BuildResult{}, err
		}

		globalNpmrcPath, err := configurationManager.DeterminePath("npmrc", context.Platform.Path, ".npmrc")
		if err != nil {
			return packit.BuildResult{}, err
		}

		if globalNpmrcPath != "" {
			err = symlinker.Link(globalNpmrcPath, filepath.Join(homeDir, ".npmrc"))
			if err != nil {
				return packit.BuildResult{}, err
			}
		}

		globalYarnrcPath, err := configurationManager.DeterminePath("yarnrc", context.Platform.Path, ".yarnrc")
		if err != nil {
			return packit.BuildResult{}, err
		}

		if globalYarnrcPath != "" {
			err = symlinker.Link(globalYarnrcPath, filepath.Join(homeDir, ".yarnrc"))
			if err != nil {
				return packit.BuildResult{}, err
			}
		}

		launch, build := entryResolver.MergeLayerTypes(PlanDependencyNodeModules, context.Plan.Entries)

		var layers []packit.Layer
		var currentModLayer string
		if build {
			layer, err := buildLayerBuilder.Build(context, currentModLayer, projectPath)
			if err != nil {
				panic(err)
			}
			currentModLayer = layer.Path

			layers = append(layers, layer)
		}

		if launch {
			layer, err := launchLayerBuilder.Build(context, currentModLayer, projectPath)
			if err != nil {
				panic(err)
			}

			layers = append(layers, layer)
		}

		err = symlinker.Unlink(filepath.Join(homeDir, ".npmrc"))
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = symlinker.Unlink(filepath.Join(homeDir, ".yarnrc"))
		if err != nil {
			return packit.BuildResult{}, err
		}

		return packit.BuildResult{
			Layers: layers,
		}, nil
	}
}
