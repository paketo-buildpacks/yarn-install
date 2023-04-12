package classic

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/packit/v2/servicebindings"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
)

//go:generate faux --interface ConfigurationManager --output fakes/configuration_manager.go
type ConfigurationManager interface {
	DeterminePath(typ, platformDir, entry string) (path string, err error)
}

type ClassicBuild struct {
	logger scribe.Emitter
}

func NewClassicBuild(logger scribe.Emitter) ClassicBuild {
	return ClassicBuild{
		logger: logger,
	}
}

func (cb ClassicBuild) Build(ctx packit.BuildContext,
	installProcess yarninstall.InstallProcess,
	sbomGenerator yarninstall.SBOMGenerator,
	symlinker yarninstall.SymlinkManager,
	entryResolver yarninstall.EntryResolver,
	projectPath, tmpDir string) (packit.BuildResult, error) {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// not tested
		log.Fatal(err)
	}

	configurationManager := NewPackageManagerConfigurationManager(servicebindings.NewResolver(), cb.logger)

	globalNpmrcPath, err := configurationManager.DeterminePath("npmrc", ctx.Platform.Path, ".npmrc")
	if err != nil {
		return packit.BuildResult{}, err
	}

	if globalNpmrcPath != "" {
		err = symlinker.Link(globalNpmrcPath, filepath.Join(homeDir, ".npmrc"))
		if err != nil {
			return packit.BuildResult{}, err
		}
	}

	globalYarnrcPath, err := configurationManager.DeterminePath("yarnrc", ctx.Platform.Path, ".yarnrc")
	if err != nil {
		return packit.BuildResult{}, err
	}

	if globalYarnrcPath != "" {
		err = symlinker.Link(globalYarnrcPath, filepath.Join(homeDir, ".yarnrc"))
		if err != nil {
			return packit.BuildResult{}, err
		}
	}

	clock := chronos.DefaultClock

	sbomDisabled, err := checkSbomDisabled()
	if err != nil {
		return packit.BuildResult{}, err
	}

	launch, build := entryResolver.MergeLayerTypes(yarninstall.PlanDependencyNodeModules, ctx.Plan.Entries)

	var layers []packit.Layer
	var currentModLayer string
	if build {
		layer, err := ctx.Layers.Get("build-modules")
		if err != nil {
			return packit.BuildResult{}, err
		}

		cb.logger.Process("Resolving installation process")

		run, sha, err := installProcess.ShouldRun(projectPath, layer.Metadata)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if run {
			cb.logger.Subprocess("Selected default build process: 'yarn install'")
			cb.logger.Break()
			cb.logger.Process("Executing build environment install process")

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

			cb.logger.Action("Completed in %s", duration.Round(time.Millisecond))
			cb.logger.Break()

			layer.Metadata = map[string]interface{}{
				"cache_sha": sha,
			}

			err = ensureNodeModulesSymlink(projectPath, layer.Path, tmpDir)
			if err != nil {
				return packit.BuildResult{}, err
			}

			path := filepath.Join(layer.Path, "node_modules", ".bin")
			layer.BuildEnv.Append("PATH", path, string(os.PathListSeparator))
			layer.BuildEnv.Override("NODE_ENV", "development")

			cb.logger.EnvironmentVariables(layer)

			if sbomDisabled {
				cb.logger.Subprocess("Skipping SBOM generation for Yarn Install")
				cb.logger.Break()

			} else {
				cb.logger.GeneratingSBOM(layer.Path)
				var sbomContent sbom.SBOM
				duration, err = clock.Measure(func() error {
					sbomContent, err = sbomGenerator.Generate(ctx.WorkingDir)
					return err
				})
				if err != nil {
					return packit.BuildResult{}, err
				}
				cb.logger.Action("Completed in %s", duration.Round(time.Millisecond))
				cb.logger.Break()

				cb.logger.FormattingSBOM(ctx.BuildpackInfo.SBOMFormats...)
				layer.SBOM, err = sbomContent.InFormats(ctx.BuildpackInfo.SBOMFormats...)
				if err != nil {
					return packit.BuildResult{}, err
				}
			}
		} else {
			cb.logger.Process("Reusing cached layer %s", layer.Path)

			err = ensureNodeModulesSymlink(projectPath, layer.Path, tmpDir)
			if err != nil {
				return packit.BuildResult{}, err
			}
		}

		layer.Build = true
		layer.Cache = true

		layers = append(layers, layer)
	}

	if launch {
		layer, err := ctx.Layers.Get("launch-modules")
		if err != nil {
			return packit.BuildResult{}, err
		}

		cb.logger.Process("Resolving installation process")

		run, sha, err := installProcess.ShouldRun(projectPath, layer.Metadata)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if run {
			cb.logger.Subprocess("Selected default build process: 'yarn install'")
			cb.logger.Break()
			cb.logger.Process("Executing launch environment install process")

			layer, err = layer.Reset()
			if err != nil {
				return packit.BuildResult{}, err
			}

			_, err = installProcess.SetupModules(projectPath, currentModLayer, layer.Path)
			if err != nil {
				return packit.BuildResult{}, err
			}

			duration, err := clock.Measure(func() error {
				return installProcess.Execute(projectPath, layer.Path, true)
			})
			if err != nil {
				return packit.BuildResult{}, err
			}

			cb.logger.Action("Completed in %s", duration.Round(time.Millisecond))
			cb.logger.Break()

			if !build {
				err = ensureNodeModulesSymlink(projectPath, layer.Path, tmpDir)
				if err != nil {
					return packit.BuildResult{}, err
				}
			}

			layer.Metadata = map[string]interface{}{
				"cache_sha": sha,
			}

			path := filepath.Join(layer.Path, "node_modules", ".bin")
			layer.LaunchEnv.Append("PATH", path, string(os.PathListSeparator))
			layer.LaunchEnv.Default("NODE_PROJECT_PATH", projectPath)

			cb.logger.EnvironmentVariables(layer)

			if sbomDisabled {
				cb.logger.Subprocess("Skipping SBOM generation for Yarn Install")
				cb.logger.Break()

			} else {
				cb.logger.GeneratingSBOM(layer.Path)
				var sbomContent sbom.SBOM
				duration, err = clock.Measure(func() error {
					sbomContent, err = sbomGenerator.Generate(ctx.WorkingDir)
					return err
				})
				if err != nil {
					return packit.BuildResult{}, err
				}
				cb.logger.Action("Completed in %s", duration.Round(time.Millisecond))
				cb.logger.Break()

				cb.logger.FormattingSBOM(ctx.BuildpackInfo.SBOMFormats...)
				layer.SBOM, err = sbomContent.InFormats(ctx.BuildpackInfo.SBOMFormats...)
				if err != nil {
					return packit.BuildResult{}, err
				}
			}

			layer.ExecD = []string{filepath.Join(ctx.CNBPath, "bin", "setup-symlinks")}

		} else {
			cb.logger.Process("Reusing cached layer %s", layer.Path)
			if !build {
				err = ensureNodeModulesSymlink(projectPath, layer.Path, tmpDir)
				if err != nil {
					return packit.BuildResult{}, err
				}
			}
		}

		layer.Launch = true

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

func ensureNodeModulesSymlink(projectDir, targetLayer, tmpDir string) error {
	projectDirNodeModules := filepath.Join(projectDir, "node_modules")
	layerNodeModules := filepath.Join(targetLayer, "node_modules")
	tmpNodeModules := filepath.Join(tmpDir, "node_modules")

	for _, d := range []string{projectDirNodeModules, tmpNodeModules} {
		err := os.RemoveAll(d)
		if err != nil {
			return err
		}
	}

	err := os.Symlink(tmpNodeModules, projectDirNodeModules)
	if err != nil {
		return err
	}

	err = os.Symlink(layerNodeModules, tmpNodeModules)
	if err != nil {
		return err
	}

	return nil
}
