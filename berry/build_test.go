package berry_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	berry "github.com/paketo-buildpacks/yarn-install/berry"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBerryBuild(t *testing.T, context spec.G, it spec.S) {

	// type linkCallParams struct {
	// 	Oldname string
	// 	Newname string
	// }

	var (
		Expect = NewWithT(t).Expect

		layersDir   string
		workingDir  string
		projectPath string
		tmpDir      string
		// homeDir     string
		cnbDir string

		ctx           packit.BuildContext
		buffer        *bytes.Buffer
		entryResolver *fakes.EntryResolver
		symlinker     *fakes.SymlinkManager
		// linkCalls           []linkCallParams
		yarnrcYmlParser     *fakes.YarnrcYmlParser
		berryInstallProcess *fakes.InstallProcess
		sbomGenerator       *fakes.SBOMGenerator
		buildProcess        berry.BerryBuild
	)

	it.Before(func() {

		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		// homeDir, err = os.MkdirTemp("", "home-dir")
		// Expect(err).NotTo(HaveOccurred())

		tmpDir, err = os.MkdirTemp("", "tmp")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())
		projectPath = filepath.Join(workingDir, "some-project-dir")

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		yarnrcYmlParser = &fakes.YarnrcYmlParser{}
		yarnrcYmlParser.ParseCall.Returns.NodeLinker = ""

		berryInstallProcess = &fakes.InstallProcess{}
		berryInstallProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
			return true, "some-awesome-shasum", nil
		}

		buffer = bytes.NewBuffer(nil)

		entryResolver = &fakes.EntryResolver{}
		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		ctx = packit.BuildContext{
			BuildpackInfo: packit.BuildpackInfo{
				Name:        "Some Buildpack",
				Version:     "1.2.3",
				SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
			},
			WorkingDir: workingDir,
			CNBPath:    cnbDir,
			Layers:     packit.Layers{Path: layersDir},
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name: "node_modules",
						Metadata: map[string]interface{}{
							"build": true,
						},
					},
				},
			},
			Stack: "some-stack",
			Platform: packit.Platform{
				Path: "some-platform-path",
			},
		}

		buildProcess = berry.NewBerryBuild(scribe.NewEmitter(buffer))
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	context("node-modules or pnpm linker", func() {
		context("build only", func() {
			it.Before(func() {
				entryResolver.MergeLayerTypesCall.Returns.Build = true
			})

			it("returns a result that installs build modules", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(result.Layers)).To(Equal(1))

				layer := result.Layers[0]
				Expect(layer.Name).To(Equal("build-modules"))
				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "build-modules")))
				Expect(layer.BuildEnv).To(Equal(packit.Environment{
					"PATH.append":       filepath.Join(layersDir, "build-modules", "node_modules", ".bin"),
					"PATH.delim":        ":",
					"NODE_ENV.override": "development",
				}))
				Expect(layer.Build).To(BeTrue())
				Expect(layer.Cache).To(BeTrue())
				Expect(layer.Metadata).To(Equal(
					map[string]interface{}{
						"cache_sha": "some-awesome-shasum",
					}))
				Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
					{
						Extension: "cdx.json",
						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
					},
					{
						Extension: "spdx.json",
						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
					},
					{
						Extension: "syft.json",
						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
					},
				}))
				Expect(len(layer.ExecD)).To(Equal(0))

				Expect(berryInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

				Expect(berryInstallProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
				Expect(berryInstallProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
				Expect(berryInstallProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))
				Expect(berryInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
				Expect(berryInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "build-modules")))
				Expect(berryInstallProcess.ExecuteCall.Receives.Launch).To(BeFalse())

				Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
			})
		})

		context("launch only", func() {
			it.Before(func() {
				entryResolver.MergeLayerTypesCall.Returns.Launch = true
			})

			it("returns a result that installs launch modules", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(result.Layers)).To(Equal(1))
				layer := result.Layers[0]
				Expect(layer.Name).To(Equal("launch-modules"))
				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
				Expect(layer.LaunchEnv).To(Equal(packit.Environment{
					"NODE_PROJECT_PATH.default": filepath.Join(workingDir, "some-project-dir"),
					"PATH.append":               filepath.Join(layersDir, "launch-modules", "node_modules", ".bin"),
					"PATH.delim":                ":",
				}))
				Expect(layer.Launch).To(BeTrue())
				Expect(layer.Metadata).To(Equal(
					map[string]interface{}{
						"cache_sha": "some-awesome-shasum",
					}))
				Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
					{
						Extension: "cdx.json",
						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
					},
					{
						Extension: "spdx.json",
						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
					},
					{
						Extension: "syft.json",
						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
					},
				}))
				Expect(len(layer.ExecD)).To(Equal(1))

				Expect(berryInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

				Expect(berryInstallProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
				Expect(berryInstallProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
				Expect(berryInstallProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

				Expect(berryInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
				Expect(berryInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "launch-modules")))
				Expect(berryInstallProcess.ExecuteCall.Receives.Launch).To(BeTrue())

				Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
			})
		})

		context("both build and launch", func() {
			type setupModulesParams struct {
				WorkingDir              string
				CurrentModulesLayerPath string
				NextModulesLayerPath    string
				TempDir                 string
			}

			var setupModulesCalls []setupModulesParams
			it.Before(func() {
				entryResolver.MergeLayerTypesCall.Returns.Launch = true
				entryResolver.MergeLayerTypesCall.Returns.Build = true
				projectPath = workingDir

				berryInstallProcess.SetupModulesCall.Stub = func(w string, c string, n string) (string, error) {
					setupModulesCalls = append(setupModulesCalls, setupModulesParams{
						WorkingDir:              w,
						CurrentModulesLayerPath: c,
						NextModulesLayerPath:    n,
					})
					return n, nil
				}
			})
			it("returns a result that has both layers and the module setup updates accordingly", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())

				launchLayer := result.Layers[1]
				Expect(launchLayer.ExecD).To(Equal([]string{filepath.Join(cnbDir, "bin", "setup-symlinks")}))
				Expect(len(result.Layers)).To(Equal(2))

				Expect(berryInstallProcess.SetupModulesCall.CallCount).To(Equal(2))

				Expect(setupModulesCalls[0].WorkingDir).To(Equal(workingDir))
				Expect(setupModulesCalls[0].CurrentModulesLayerPath).To(Equal(""))
				Expect(setupModulesCalls[0].NextModulesLayerPath).To(Equal(result.Layers[0].Path))

				Expect(setupModulesCalls[1].WorkingDir).To(Equal(workingDir))
				Expect(setupModulesCalls[1].CurrentModulesLayerPath).To(Equal(result.Layers[0].Path))
				Expect(setupModulesCalls[1].NextModulesLayerPath).To(Equal(result.Layers[1].Path))
			})

		})

		context("neither build nor launch", func() {
			it("returns a result that has no layers", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(packit.BuildResult{}))
			})
		})

		context("when re-using previous modules layer", func() {
			it.Before(func() {
				berryInstallProcess.ShouldRunCall.Stub = nil
				berryInstallProcess.ShouldRunCall.Returns.Run = false
				entryResolver.MergeLayerTypesCall.Returns.Launch = true
				entryResolver.MergeLayerTypesCall.Returns.Build = true
			})

			it("does not redo the build process", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(result.Layers)).To(Equal(2))
				buildLayer := result.Layers[0]
				Expect(buildLayer.Name).To(Equal("build-modules"))
				Expect(buildLayer.Path).To(Equal(filepath.Join(layersDir, "build-modules")))
				Expect(buildLayer.Build).To(BeTrue())
				Expect(buildLayer.Cache).To(BeTrue())

				launchLayer := result.Layers[1]
				Expect(launchLayer.Name).To(Equal("launch-modules"))
				Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
				Expect(launchLayer.Launch).To(BeTrue())

				workspaceLink, err := os.Readlink(filepath.Join(workingDir, "some-project-dir", "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(workspaceLink).To(Equal(filepath.Join(tmpDir, "node_modules")))

				tmpLink, err := os.Readlink(filepath.Join(tmpDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpLink).To(Equal(filepath.Join(layersDir, "build-modules", "node_modules")))

			})
		})

		context("when re-using previous launch-modules layer", func() {
			it.Before(func() {
				berryInstallProcess.ShouldRunCall.Stub = nil
				berryInstallProcess.ShouldRunCall.Returns.Run = false
				entryResolver.MergeLayerTypesCall.Returns.Launch = true
			})

			it("does not redo the build process", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(result.Layers)).To(Equal(1))
				launchLayer := result.Layers[0]
				Expect(launchLayer.Name).To(Equal("launch-modules"))
				Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
				Expect(launchLayer.Launch).To(BeTrue())

				workspaceLink, err := os.Readlink(filepath.Join(workingDir, "some-project-dir", "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(workspaceLink).To(Equal(filepath.Join(tmpDir, "node_modules")))

				tmpLink, err := os.Readlink(filepath.Join(tmpDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(tmpLink).To(Equal(filepath.Join(layersDir, "launch-modules", "node_modules")))
			})
		})

	})

	context("pnp linker", func() {
		context.Focus("launch only", func() {
			it.Before(func() {
				yarnrcYmlParser.ParseCall.Returns.NodeLinker = "pnp"
				entryResolver.MergeLayerTypesCall.Returns.Launch = true
				entryResolver.MergeLayerTypesCall.Returns.Build = false
			})

			it("returns a result that installs packages", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(result.Layers)).To(Equal(1))

				layer := result.Layers[0]
				Expect(layer.Name).To(Equal("pkgs"))
				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "pkgs")))

				//TODO: Is there a node_modules/.bin equivalent?
				// Expect(layer.BuildEnv).To(Equal(packit.Environment{
				// 	"PATH.append":       filepath.Join(layersDir, "launch-pkgs", "node_modules", ".bin"),
				// 	"PATH.delim":        ":",
				// 	"NODE_ENV.override": "development",
				// }))

				Expect(layer.Launch).To(BeTrue())
				Expect(layer.Metadata).To(Equal(
					map[string]interface{}{
						"cache_sha": "some-awesome-shasum",
					}))
				// Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
				// 	{
				// 		Extension: "cdx.json",
				// 		Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
				// 	},
				// 	{
				// 		Extension: "spdx.json",
				// 		Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
				// 	},
				// 	{
				// 		Extension: "syft.json",
				// 		Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
				// 	},
				// }))
				//TODO: Do we need this?
				Expect(len(layer.ExecD)).To(Equal(0))

				Expect(berryInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

				Expect(berryInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
				Expect(berryInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "pkgs")))
				Expect(berryInstallProcess.ExecuteCall.Receives.Launch).To(BeTrue())

				// Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
			})
		})
		context.Focus("build only", func() {
			it.Before(func() {
				yarnrcYmlParser.ParseCall.Returns.NodeLinker = ""
				entryResolver.MergeLayerTypesCall.Returns.Launch = false
				entryResolver.MergeLayerTypesCall.Returns.Build = true
			})
			it("returns a result that installs build modules", func() {
				result, err := buildProcess.Build(
					ctx,
					berryInstallProcess,
					sbomGenerator,
					symlinker,
					entryResolver,
					projectPath,
					tmpDir)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(result.Layers)).To(Equal(1))

				layer := result.Layers[0]
				Expect(layer.Name).To(Equal("pkgs"))
				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "pkgs")))

				//TODO: Is there a node_modules/.bin equivalent?
				// Expect(layer.BuildEnv).To(Equal(packit.Environment{
				// 	"PATH.append":       filepath.Join(layersDir, "launch-pkgs", "node_modules", ".bin"),
				// 	"PATH.delim":        ":",
				// 	"NODE_ENV.override": "development",
				// }))

				Expect(layer.Build).To(BeTrue())
				Expect(layer.Build).To(BeTrue())
				Expect(layer.Launch).To(BeFalse())
				Expect(layer.Metadata).To(Equal(
					map[string]interface{}{
						"cache_sha": "some-awesome-shasum",
					}))
				// Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
				// 	{
				// 		Extension: "cdx.json",
				// 		Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
				// 	},
				// 	{
				// 		Extension: "spdx.json",
				// 		Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
				// 	},
				// 	{
				// 		Extension: "syft.json",
				// 		Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
				// 	},
				// }))
				//TODO: Do we need this?
				// Expect(len(layer.ExecD)).To(Equal(1))

				Expect(berryInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

				Expect(berryInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
				Expect(berryInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "pkgs")))
				Expect(berryInstallProcess.ExecuteCall.Receives.Launch).To(BeFalse())

				// Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
			})
		})
		context("both build and launch", func() {})
		context("neither build nor launch", func() {})
	})

}