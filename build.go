package yarninstall

import (
	"os"
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/paketo-buildpacks/packit/scribe"
)

//go:generate faux --interface CacheMatcher --output fakes/cache_matcher.go
type CacheMatcher interface {
	Match(metadata map[string]interface{}, key, sha string) bool
}

//go:generate faux --interface DependencyService --output fakes/dependency_service.go
type DependencyService interface {
	Resolve(path, name, version, stack string) (postal.Dependency, error)
	Install(dependency postal.Dependency, cnbPath, layerPath string) error
}

//go:generate faux --interface InstallProcess --output fakes/install_process.go
type InstallProcess interface {
	ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error)
	Execute(workingDir, modulesLayerPath string) error
}

func Build(dependencyService DependencyService, cacheMatcher CacheMatcher, installProcess InstallProcess, clock chronos.Clock, logger scribe.Logger) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		logger.Process("Resolving installation process")

		modulesLayer, err := context.Layers.Get("modules")
		if err != nil {
			return packit.BuildResult{}, err
		}

		modulesLayer = setLayerFlags(modulesLayer, context.Plan.Entries)

		run, sha, err := installProcess.ShouldRun(context.WorkingDir, modulesLayer.Metadata)
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
				return installProcess.Execute(context.WorkingDir, modulesLayer.Path)
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

			path := filepath.Join(modulesLayer.Path, "node_modules", ".bin")
			modulesLayer.SharedEnv.Append("PATH", path, string(os.PathListSeparator))

			logger.Process("Configuring environment")
			logger.Subprocess("%s", scribe.NewFormattedMapFromEnvironment(modulesLayer.SharedEnv))
		} else {
			logger.Process("Reusing cached layer %s", modulesLayer.Path)

			err := os.RemoveAll(filepath.Join(context.WorkingDir, "node_modules"))
			if err != nil {
				return packit.BuildResult{}, err
			}
			err = os.Symlink(filepath.Join(modulesLayer.Path, "node_modules"), filepath.Join(context.WorkingDir, "node_modules"))
			if err != nil {
				// not tested
				return packit.BuildResult{}, err
			}
		}

		logger.Break()

		return packit.BuildResult{
			Plan: context.Plan,
			Layers: []packit.Layer{
				modulesLayer,
			},
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
