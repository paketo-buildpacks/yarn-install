package yarninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/scribe"
  "github.com/paketo-buildpacks/packit/v2/servicebindings"
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
	homeDir string,
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

		globalNpmrcPath, err := getBinding("npmrc", "", context.Platform.Path, ".npmrc", bindingResolver, logger)
		if err != nil {
			return packit.BuildResult{}, err
		}
		if globalNpmrcPath != "" {
			err = symlinker.Link(globalNpmrcPath, filepath.Join(homeDir, ".npmrc"))
			if err != nil {
				return packit.BuildResult{}, err
			}
		}

		globalYarnrcPath, err := getBinding("yarnrc", "", context.Platform.Path, ".yarnrc", bindingResolver, logger)
		if err != nil {
			return packit.BuildResult{}, err
		}
		if globalYarnrcPath != "" {
			err = symlinker.Link(globalYarnrcPath, filepath.Join(homeDir, ".yarnrc"))
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

		err = symlinker.Unlink(filepath.Join(homeDir, ".npmrc"))
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = symlinker.Unlink(filepath.Join(homeDir, ".yarnrc"))
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

func getBinding(typ, provider, bindingsRoot, entry string, bindingResolver BindingResolver, logger scribe.Logger) (configPath string, err error) {
	bindings, err := bindingResolver.Resolve(typ, provider, bindingsRoot)
	if err != nil {
		return "", err
	}

	if len(bindings) > 1 {
		return "", fmt.Errorf("binding resolver found more than one binding of type '%s'", typ)
	}

	if len(bindings) == 1 {
		logger.Process("Loading service binding of type '%s'", typ)

		fileExists := false
		for key := range bindings[0].Entries {
			if key == entry {
				fileExists = true
				break
			}
		}
		if !fileExists {
			return "", fmt.Errorf("binding of type '%s' does not contain required entry '%s'", typ, entry)
		}
		return filepath.Join(bindings[0].Path, entry), nil
	}
	return "", nil
}
