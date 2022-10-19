package yarninstall

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

func Build(pathParser PathParser,
	entryResolver EntryResolver,
	configurationManager ConfigurationManager,
	homeDir string,
	symlinker SymlinkManager,
	installProcess InstallProcess,
	sbomGenerator SBOMGenerator,
	clock chronos.Clock,
	logger scribe.Emitter,
	tmpDir string) packit.BuildFunc {
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

		sbomDisabled, err := checkSbomDisabled()
		if err != nil {
			return packit.BuildResult{}, err
		}

		var layers []packit.Layer
		var currentModLayer string
		if build {
			layer, err := context.Layers.Get("build-modules")
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Process("Resolving installation process")

			run, sha, err := installProcess.ShouldRun(projectPath, layer.Metadata)
			if err != nil {
				return packit.BuildResult{}, err
			}

			if run {
				logger.Subprocess("Selected default build process: 'yarn install'")
				logger.Break()
				logger.Process("Executing build environment install process")

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

				logger.Action("Completed in %s", duration.Round(time.Millisecond))
				logger.Break()

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

				logger.EnvironmentVariables(layer)

				if sbomDisabled {
					logger.Subprocess("Skipping SBOM generation for Yarn Install")
					logger.Break()

				} else {
					logger.GeneratingSBOM(layer.Path)
					var sbomContent sbom.SBOM
					duration, err = clock.Measure(func() error {
						sbomContent, err = sbomGenerator.Generate(context.WorkingDir)
						return err
					})
					if err != nil {
						return packit.BuildResult{}, err
					}
					logger.Action("Completed in %s", duration.Round(time.Millisecond))
					logger.Break()

					logger.FormattingSBOM(context.BuildpackInfo.SBOMFormats...)
					layer.SBOM, err = sbomContent.InFormats(context.BuildpackInfo.SBOMFormats...)
					if err != nil {
						return packit.BuildResult{}, err
					}
				}
			} else {
				logger.Process("Reusing cached layer %s", layer.Path)

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
			layer, err := context.Layers.Get("launch-modules")
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Process("Resolving installation process")

			run, sha, err := installProcess.ShouldRun(projectPath, layer.Metadata)
			if err != nil {
				return packit.BuildResult{}, err
			}

			if run {
				logger.Subprocess("Selected default build process: 'yarn install'")
				logger.Break()
				logger.Process("Executing launch environment install process")

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

				logger.Action("Completed in %s", duration.Round(time.Millisecond))
				logger.Break()

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

				logger.EnvironmentVariables(layer)

				if sbomDisabled {
					logger.Subprocess("Skipping SBOM generation for Yarn Install")
					logger.Break()

				} else {
					logger.GeneratingSBOM(layer.Path)
					var sbomContent sbom.SBOM
					duration, err = clock.Measure(func() error {
						sbomContent, err = sbomGenerator.Generate(context.WorkingDir)
						return err
					})
					if err != nil {
						return packit.BuildResult{}, err
					}
					logger.Action("Completed in %s", duration.Round(time.Millisecond))
					logger.Break()

					logger.FormattingSBOM(context.BuildpackInfo.SBOMFormats...)
					layer.SBOM, err = sbomContent.InFormats(context.BuildpackInfo.SBOMFormats...)
					if err != nil {
						return packit.BuildResult{}, err
					}
				}

				layer.ExecD = []string{filepath.Join(context.CNBPath, "bin", "setup-symlinks")}

			} else {
				logger.Process("Reusing cached layer %s", layer.Path)
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
