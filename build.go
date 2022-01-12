package yarninstall

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/scribe"
	"github.com/paketo-buildpacks/packit/servicebindings"
)

//go:generate faux --interface SymlinkManager --output fakes/symlink_manager.go
type SymlinkManager interface {
	Link(oldname, newname string) error
	Unlink(path string) error
}

//go:generate faux --interface InstallProcess --output fakes/install_process.go
type InstallProcess interface {
	ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error)
	Execute(workingDir, modulesLayerPath string) error
}

//go:generate faux --interface BindingResolver --output fakes/binding_resolver.go
type BindingResolver interface {
	Resolve(typ, provider, platformDir string) ([]servicebindings.Binding, error)
}

func Build(pathParser PathParser,
	bindingResolver BindingResolver,
	symlinker SymlinkManager,
	installProcess InstallProcess,
	clock chronos.Clock,
	logger scribe.Logger) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		logger.Process("Resolving installation process")

		modulesLayer, err := context.Layers.Get("modules")
		if err != nil {
			return packit.BuildResult{}, err
		}

		modulesLayer = setLayerFlags(modulesLayer, context.Plan.Entries)

		projectPath, err := pathParser.Get(context.WorkingDir)
		if err != nil {
			return packit.BuildResult{}, err
		}

		var globalNpmrcPath string
		bindings, err := bindingResolver.Resolve("npmrc", "", context.Platform.Path)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if len(bindings) > 1 {
			return packit.BuildResult{}, errors.New("binding resolver found more than one binding of type 'npmrc'")
		}

		if len(bindings) == 1 {
			logger.Process("Loading npmrc service binding")

			npmrcExists := false
			for key := range bindings[0].Entries {
				if key == ".npmrc" {
					npmrcExists = true
					break
				}
			}
			if !npmrcExists {
				return packit.BuildResult{}, errors.New("binding of type 'npmrc' does not contain required entry '.npmrc'")
			}
			globalNpmrcPath = filepath.Join(bindings[0].Path, ".npmrc")
		}

		home, err := os.UserHomeDir()
		if err != nil {
			// not tested
			return packit.BuildResult{}, err
		}

		if globalNpmrcPath != "" {
			err = symlinker.Link(globalNpmrcPath, filepath.Join(home, ".npmrc"))
			if err != nil {
				return packit.BuildResult{}, err
			}
		}

		run, sha, err := installProcess.ShouldRun(projectPath, modulesLayer.Metadata)
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Subprocess("Selected default build process: 'yarn install'")
		logger.Break()

		if run {
			logger.Process("Executing build process")

			modulesLayer, err = modulesLayer.Reset()
			if err != nil {
				return packit.BuildResult{}, err
			}

			modulesLayer = setLayerFlags(modulesLayer, context.Plan.Entries)

			duration, err := clock.Measure(func() error {
				return installProcess.Execute(projectPath, modulesLayer.Path)
			})
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Action("Completed in %s", duration.Round(time.Millisecond))
			logger.Break()

			modulesLayer.Metadata = map[string]interface{}{
				"built_at":  clock.Now().Format(time.RFC3339Nano),
				"cache_sha": sha,
			}

			logger.Process("Configuring environment")

			path := filepath.Join(modulesLayer.Path, "node_modules", ".bin")
			modulesLayer.SharedEnv.Append("PATH", path, string(os.PathListSeparator))
			logger.Subprocess("%s", scribe.NewFormattedMapFromEnvironment(modulesLayer.SharedEnv))

			if globalNpmrcPath != "" {
				modulesLayer.BuildEnv.Default("NPM_CONFIG_GLOBALCONFIG", globalNpmrcPath)
				logger.Subprocess("%s", scribe.NewFormattedMapFromEnvironment(modulesLayer.BuildEnv))
			}
		} else {
			logger.Process("Reusing cached layer %s", modulesLayer.Path)

			err := os.RemoveAll(filepath.Join(projectPath, "node_modules"))
			if err != nil {
				return packit.BuildResult{}, err
			}

			err = symlinker.Link(filepath.Join(modulesLayer.Path, "node_modules"), filepath.Join(projectPath, "node_modules"))
			if err != nil {
				return packit.BuildResult{}, err
			}
		}

		logger.Break()

		err = symlinker.Unlink(filepath.Join(home, ".npmrc"))
		if err != nil {
			return packit.BuildResult{}, err
		}

		return packit.BuildResult{
			Layers: []packit.Layer{modulesLayer},
		}, nil
	}
}

func setLayerFlags(layer packit.Layer, entries []packit.BuildpackPlanEntry) packit.Layer {
	for _, entry := range entries {
		launch, ok := entry.Metadata["launch"].(bool)
		if ok && launch {
			layer.Launch = true
			layer.Cache = true
		}
	}

	for _, entry := range entries {
		build, ok := entry.Metadata["build"].(bool)
		if ok && build {
			layer.Build = true
			layer.Cache = true
		}
	}

	return layer
}
