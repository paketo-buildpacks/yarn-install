package yarn

import (
	"os"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/packit/postal"
	"github.com/cloudfoundry/packit/scribe"
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
	Execute(workingDir, modulesLayerPath, yarnLayerPath string) error
}

func Build(dependencyService DependencyService, cacheMatcher CacheMatcher, installProcess InstallProcess, clock Clock, logger scribe.Logger) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		yarnLayer, err := context.Layers.Get("yarn", packit.LaunchLayer, packit.CacheLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		dependency, err := dependencyService.Resolve(filepath.Join(context.CNBPath, "buildpack.toml"), "yarn", "*", context.Stack)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if !cacheMatcher.Match(yarnLayer.Metadata, "cache_sha", dependency.SHA256) {
			logger.Process("Executing build process")

			err = yarnLayer.Reset()
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Subprocess("Installing Yarn %s", dependency.Version)
			then := clock.Now()

			err = dependencyService.Install(dependency, context.CNBPath, yarnLayer.Path)
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Action("Completed in %s", time.Since(then).Round(time.Millisecond))
			logger.Break()

			yarnLayer.Metadata = map[string]interface{}{
				"built_at":  clock.Now().Format(time.RFC3339Nano),
				"cache_sha": dependency.SHA256,
			}
		} else {
			logger.Process("Reusing cached layer %s", yarnLayer.Path)
			logger.Break()
		}

		logger.Process("Resolving installation process")

		modulesLayer, err := context.Layers.Get("modules", packit.LaunchLayer, packit.CacheLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		run, sha, err := installProcess.ShouldRun(context.WorkingDir, modulesLayer.Metadata)
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Subprocess("Selected default build process: 'yarn install'")
		logger.Break()

		if run {
			logger.Process("Executing build process")

			err = modulesLayer.Reset()
			if err != nil {
				return packit.BuildResult{}, err
			}

			then := clock.Now()

			err = installProcess.Execute(context.WorkingDir, modulesLayer.Path, yarnLayer.Path)
			if err != nil {
				return packit.BuildResult{}, err
			}
			logger.Action("Completed in %s", time.Since(then).Round(time.Millisecond))
			logger.Break()

			modulesLayer.Metadata = map[string]interface{}{
				"built_at":  clock.Now().Format(time.RFC3339Nano),
				"cache_sha": sha,
			}
		} else {
			logger.Process("Reusing cached layer %s", modulesLayer.Path)

			err := os.RemoveAll(filepath.Join(context.WorkingDir, "node_modules"))
			if err != nil {
				return packit.BuildResult{}, err
			}

			err = os.Symlink(filepath.Join(modulesLayer.Path, "node_modules"), filepath.Join(context.WorkingDir, "node_modules"))
			if err != nil {
				return packit.BuildResult{}, err
			}
		}

		return packit.BuildResult{
			Plan: context.Plan,
			Layers: []packit.Layer{
				yarnLayer,
				modulesLayer,
			},
			Processes: []packit.Process{
				{
					Type:    "web",
					Command: "yarn start",
				},
			},
		}, nil
	}
}
