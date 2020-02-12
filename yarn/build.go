package yarn

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/packit/postal"
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
	Execute(workingDir, layerPath string) error
}

func Build(dependencyService DependencyService, cacheMatcher CacheMatcher, installProcess InstallProcess, clock Clock) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		yarnLayer, err := context.Layers.Get("yarn", packit.LaunchLayer, packit.CacheLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}
		os.Setenv("PATH", fmt.Sprintf("%s%c%s", os.Getenv("PATH"), os.PathListSeparator, filepath.Join(yarnLayer.Path, "bin")))

		dependency, err := dependencyService.Resolve(filepath.Join(context.CNBPath, "buildpack.toml"), "yarn", "*", context.Stack)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if !cacheMatcher.Match(yarnLayer.Metadata, "cache_sha", dependency.SHA256) {
			err = dependencyService.Install(dependency, context.CNBPath, yarnLayer.Path)
			if err != nil {
				return packit.BuildResult{}, err
			}

			yarnLayer.Metadata = map[string]interface{}{
				"built_at":  clock.Now().Format(time.RFC3339Nano),
				"cache_sha": dependency.SHA256,
			}
		}

		modulesLayer, err := context.Layers.Get("modules", packit.LaunchLayer, packit.CacheLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		run, sha, err := installProcess.ShouldRun(context.WorkingDir, modulesLayer.Metadata)
		if err != nil {
			panic(err)
		}

		if run {
			err = modulesLayer.Reset()
			if err != nil {
				return packit.BuildResult{}, err
			}

			err = installProcess.Execute(context.WorkingDir, modulesLayer.Path)
			if err != nil {
				return packit.BuildResult{}, err
			}

			modulesLayer.Metadata = map[string]interface{}{
				"built_at":  clock.Now().Format(time.RFC3339Nano),
				"cache_sha": sha,
			}
		} else {
			err := os.RemoveAll(filepath.Join(context.WorkingDir, "node_modules"))
			if err != nil {
				panic(err)
			}

			err = os.Symlink(filepath.Join(modulesLayer.Path, "node_modules"), filepath.Join(context.WorkingDir, "node_modules"))
			if err != nil {
				panic(err)
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
