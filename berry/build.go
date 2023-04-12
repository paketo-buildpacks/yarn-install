package berry

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

type BerryBuild struct {
	logger scribe.Emitter
}

func NewBerryBuild(logger scribe.Emitter) BerryBuild {
	return BerryBuild{
		logger: logger,
	}
}

func (bb BerryBuild) Build(ctx packit.BuildContext,
	installProcess yarninstall.InstallProcess,
	sbomGenerator yarninstall.SBOMGenerator,
	symlinker yarninstall.SymlinkManager,
	entryResolver yarninstall.EntryResolver,
	projectPath, tmpDir string) (packit.BuildResult, error) {

	clock := chronos.DefaultClock

	sbomDisabled, err := checkSbomDisabled()
	if err != nil {
		return packit.BuildResult{}, err
	}

	_, build := entryResolver.MergeLayerTypes(yarninstall.PlanDependencyNodeModules, ctx.Plan.Entries)

	var layers []packit.Layer
	var currentModLayer string
	if build {
		layer, err := ctx.Layers.Get("build-modules")
		if err != nil {
			return packit.BuildResult{}, err
		}

		bb.logger.Process("Resolving installation process")

		run, sha, err := installProcess.ShouldRun(projectPath, layer.Metadata)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if run {
			bb.logger.Subprocess("Selected default build process: 'yarn install'")
			bb.logger.Break()
			bb.logger.Process("Executing build environment install process")

			layer, err = layer.Reset()
			if err != nil {
				return packit.BuildResult{}, err
			}

			currentModLayer, err = installProcess.SetupModules(projectPath, currentModLayer, layer.Path)
			if err != nil {
				return packit.BuildResult{}, err
			}

			duration, err := clock.Measure(func() error {
				return installProcess.Execute(projectPath, layer.Path, false)
			})
			if err != nil {
				return packit.BuildResult{}, err
			}

			bb.logger.Action("Completed in %s", duration.Round(time.Millisecond))
			bb.logger.Break()

			layer.Metadata = map[string]interface{}{
				"cache_sha": sha,
			}

			// 	err = ensureNodeModulesSymlink(projectPath, layer.Path, tmpDir)
			// 	if err != nil {
			// 		return packit.BuildResult{}, err
			// 	}

			path := filepath.Join(layer.Path, "node_modules", ".bin")
			layer.BuildEnv.Append("PATH", path, string(os.PathListSeparator))
			layer.BuildEnv.Override("NODE_ENV", "development")

			// 	bb.logger.EnvironmentVariables(layer)

			if sbomDisabled {
				bb.logger.Subprocess("Skipping SBOM generation for Yarn Install")
				bb.logger.Break()

			} else {
				bb.logger.GeneratingSBOM(layer.Path)
				var sbomContent sbom.SBOM
				duration, err = clock.Measure(func() error {
					sbomContent, err = sbomGenerator.Generate(ctx.WorkingDir)
					return err
				})
				if err != nil {
					return packit.BuildResult{}, err
				}
				bb.logger.Action("Completed in %s", duration.Round(time.Millisecond))
				bb.logger.Break()

				bb.logger.FormattingSBOM(ctx.BuildpackInfo.SBOMFormats...)
				layer.SBOM, err = sbomContent.InFormats(ctx.BuildpackInfo.SBOMFormats...)
				if err != nil {
					return packit.BuildResult{}, err
				}
			}
		} else {
			// 	bb.logger.Process("Reusing cached layer %s", layer.Path)

			// 	err = ensureNodeModulesSymlink(projectPath, layer.Path, tmpDir)
			// 	if err != nil {
			// 		return packit.BuildResult{}, err
			// 	}
		}

		layer.Build = true
		layer.Cache = true

		layers = append(layers, layer)
	}

	return packit.BuildResult{
		Layers: layers,
	}, nil
}

func checkSbomDisabled() (bool, error) {
	if disableStr, ok := os.LookupEnv("BP_DISABLE_SBOM"); ok {
		disable, err := strconv.ParseBool(disableStr)
		if err != nil {
			return false, fmt.Errorf("failed to parse BP_DISABLE_SBOM value %s: %w", disableStr, err)
		}
		return disable, nil
	}
	return false, nil
}
