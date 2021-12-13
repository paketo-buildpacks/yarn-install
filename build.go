package yarninstall

import (
	"os"
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface InstallProcess --output fakes/install_process.go
type InstallProcess interface {
	ShouldRun(workingDir string, metadata map[string]interface{}) (run bool, sha string, err error)
	Execute(workingDir, modulesLayerPath string) error
}

//go:generate faux --interface SBOMGenerator --output fakes/sbom_generator.go
type SBOMGenerator interface {
	Generate(dir string) (sbom.SBOM, error)
}

func Build(pathParser PathParser, installProcess InstallProcess, clock chronos.Clock, logger scribe.Logger, sbomGenerator SBOMGenerator) packit.BuildFunc {
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

			path := filepath.Join(modulesLayer.Path, "node_modules", ".bin")
			modulesLayer.SharedEnv.Append("PATH", path, string(os.PathListSeparator))

			logger.Process("Configuring environment")
			logger.Subprocess("%s", scribe.NewFormattedMapFromEnvironment(modulesLayer.SharedEnv))
			logger.Break()

			logger.Process("Generating SBOM")

			var sbomContent sbom.SBOM
			duration, err = clock.Measure(func() error {
				sbomContent, err = sbomGenerator.Generate(context.WorkingDir)
				return err
			})
			if err != nil {
				return packit.BuildResult{}, err
			}
			logger.Action("Completed in %s", duration.Round(time.Millisecond))

			modulesLayer.SBOM, err = sbomContent.InFormats(context.BuildpackInfo.SBOMFormats...)
			if err != nil {
				return packit.BuildResult{}, err
			}
		} else {
			logger.Process("Reusing cached layer %s", modulesLayer.Path)

			err := os.RemoveAll(filepath.Join(projectPath, "node_modules"))
			if err != nil {
				return packit.BuildResult{}, err
			}
			err = os.Symlink(filepath.Join(modulesLayer.Path, "node_modules"), filepath.Join(projectPath, "node_modules"))
			if err != nil {
				// not tested
				return packit.BuildResult{}, err
			}
		}

		logger.Break()

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
