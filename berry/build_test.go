package berry_test

// import (
// 	"bytes"
// 	"os"
// 	"path/filepath"
// 	"testing"

// 	"github.com/paketo-buildpacks/packit/v2"
// 	"github.com/paketo-buildpacks/packit/v2/chronos"
// 	"github.com/paketo-buildpacks/packit/v2/sbom"
// 	"github.com/paketo-buildpacks/packit/v2/scribe"
// 	yarninstall "github.com/paketo-buildpacks/yarn-install"
// 	"github.com/paketo-buildpacks/yarn-install/fakes"
// 	"github.com/sclevine/spec"

// 	. "github.com/onsi/gomega"
// )

// func testOrchestrator(t *testing.T, context spec.G, it spec.S) {
// 	type determinePathCallParams struct {
// 		Typ         string
// 		PlatformDir string
// 		Entry       string
// 	}

// 	type linkCallParams struct {
// 		Oldname string
// 		Newname string
// 	}

// 	var (
// 		Expect = NewWithT(t).Expect

// 		layersDir  string
// 		workingDir string
// 		homeDir    string
// 		cnbDir     string

// 		determinePathCalls    []determinePathCallParams
// 		configurationManager  *fakes.ConfigurationManager
// 		buffer                *bytes.Buffer
// 		entryResolver         *fakes.EntryResolver
// 		yarnrcYmlParser       *fakes.YarnrcYmlParser
// 		classicInstallProcess *fakes.InstallProcess
// 		berryInstallProcess   *fakes.InstallProcess
// 		linkCalls             []linkCallParams
// 		pathParser            *fakes.PathParser
// 		sbomGenerator         *fakes.SBOMGenerator
// 		symlinker             *fakes.SymlinkManager
// 		unlinkPaths           []string
// 		build                 packit.BuildFunc
// 	)

// 	it.Before(func() {
// 		var err error
// 		layersDir, err = os.MkdirTemp("", "layers")
// 		Expect(err).NotTo(HaveOccurred())

// 		workingDir, err = os.MkdirTemp("", "working-dir")
// 		Expect(err).NotTo(HaveOccurred())

// 		homeDir, err = os.MkdirTemp("", "home-dir")
// 		Expect(err).NotTo(HaveOccurred())

// 		Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())

// 		cnbDir, err = os.MkdirTemp("", "cnb")
// 		Expect(err).NotTo(HaveOccurred())

// 		classicInstallProcess = &fakes.InstallProcess{}
// 		classicInstallProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
// 			return true, "some-awesome-shasum", nil
// 		}

// 		berryInstallProcess = &fakes.InstallProcess{}
// 		berryInstallProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
// 			return true, "some-awesome-shasum", nil
// 		}

// 		entryResolver = &fakes.EntryResolver{}

// 		buffer = bytes.NewBuffer(nil)

// 		pathParser = &fakes.PathParser{}
// 		pathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "some-project-dir")

// 		yarnrcYmlParser = &fakes.YarnrcYmlParser{}
// 		yarnrcYmlParser.ParseLinkerCall.Returns.Err = os.ErrNotExist

// 		sbomGenerator = &fakes.SBOMGenerator{}
// 		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

// 		configurationManager = &fakes.ConfigurationManager{}

// 		configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
// 			determinePathCalls = append(determinePathCalls, determinePathCallParams{
// 				Typ:         typ,
// 				Entry:       entry,
// 				PlatformDir: platform,
// 			})
// 			return "", nil
// 		}
// 		symlinker = &fakes.SymlinkManager{}
// 		symlinker.LinkCall.Stub = func(o, n string) error {
// 			linkCalls = append(linkCalls, linkCallParams{
// 				Oldname: o,
// 				Newname: n,
// 			})
// 			return nil
// 		}
// 		symlinker.UnlinkCall.Stub = func(p string) error {
// 			unlinkPaths = append(unlinkPaths, p)
// 			return nil
// 		}

// 		build = yarninstall.Build(
// 			pathParser,
// 			yarnrcYmlParser,
// 			entryResolver,
// 			configurationManager,
// 			homeDir,
// 			symlinker,
// 			classicInstallProcess,
// 			berryInstallProcess,
// 			sbomGenerator,
// 			chronos.DefaultClock,
// 			scribe.NewEmitter(buffer),
// 		)
// 	})

// 	it.After(func() {
// 		Expect(os.RemoveAll(layersDir)).To(Succeed())
// 		Expect(os.RemoveAll(workingDir)).To(Succeed())
// 		Expect(os.RemoveAll(cnbDir)).To(Succeed())
// 	})

// 	context("when node_modules is required during build", func() {
// 		it.Before(func() {
// 			entryResolver.MergeLayerTypesCall.Returns.Build = true
// 		})

// 		context("Classic", func() {
// 			it("returns a result that installs build modules", func() {
// 				result, err := build(packit.BuildContext{
// 					BuildpackInfo: packit.BuildpackInfo{
// 						Name:        "Some Buildpack",
// 						Version:     "1.2.3",
// 						SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 					},
// 					WorkingDir: workingDir,
// 					CNBPath:    cnbDir,
// 					Layers:     packit.Layers{Path: layersDir},
// 					Plan: packit.BuildpackPlan{
// 						Entries: []packit.BuildpackPlanEntry{
// 							{
// 								Name: "node_modules",
// 								Metadata: map[string]interface{}{
// 									"build": true,
// 								},
// 							},
// 						},
// 					},
// 					Stack: "some-stack",
// 					Platform: packit.Platform{
// 						Path: "some-platform-path",
// 					},
// 				})
// 				Expect(err).NotTo(HaveOccurred())

// 				Expect(len(result.Layers)).To(Equal(1))

// 				layer := result.Layers[0]
// 				Expect(layer.Name).To(Equal("build-modules"))
// 				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "build-modules")))
// 				Expect(layer.BuildEnv).To(Equal(packit.Environment{
// 					"PATH.append":       filepath.Join(layersDir, "build-modules", "node_modules", ".bin"),
// 					"PATH.delim":        ":",
// 					"NODE_ENV.override": "development",
// 				}))
// 				Expect(layer.Build).To(BeTrue())
// 				Expect(layer.Cache).To(BeTrue())
// 				Expect(layer.Metadata).To(Equal(
// 					map[string]interface{}{
// 						"cache_sha": "some-awesome-shasum",
// 					}))
// 				Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
// 					{
// 						Extension: "cdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
// 					},
// 					{
// 						Extension: "spdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
// 					},
// 					{
// 						Extension: "syft.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
// 					},
// 				}))
// 				Expect(len(layer.ExecD)).To(Equal(0))

// 				Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

// 				Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

// 				Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
// 				Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
// 				Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

// 				Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
// 				Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
// 				Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

// 				Expect(symlinker.LinkCall.CallCount).To(BeZero())

// 				Expect(classicInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

// 				Expect(classicInstallProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(workingDir))
// 				Expect(classicInstallProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
// 				Expect(classicInstallProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))
// 				Expect(classicInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
// 				Expect(classicInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "build-modules")))
// 				Expect(classicInstallProcess.ExecuteCall.Receives.Launch).To(BeFalse())

// 				Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
// 			})
// 		})

// 		context("Berry", func() {
// 			it.Before(func() {
// 				yarnrcYmlParser.ParseLinkerCall.Returns.Err = nil
// 				entryResolver.MergeLayerTypesCall.Returns.Build = true
// 			})

// 			it.Focus("returns a result that installs build modules", func() {
// 				result, err := build(packit.BuildContext{
// 					BuildpackInfo: packit.BuildpackInfo{
// 						Name:        "Some Buildpack",
// 						Version:     "1.2.3",
// 						SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 					},
// 					WorkingDir: workingDir,
// 					CNBPath:    cnbDir,
// 					Layers:     packit.Layers{Path: layersDir},
// 					Plan: packit.BuildpackPlan{
// 						Entries: []packit.BuildpackPlanEntry{
// 							{
// 								Name: "node_modules",
// 								Metadata: map[string]interface{}{
// 									"build": true,
// 								},
// 							},
// 						},
// 					},
// 					Stack: "some-stack",
// 					Platform: packit.Platform{
// 						Path: "some-platform-path",
// 					},
// 				})
// 				Expect(err).NotTo(HaveOccurred())

// 				Expect(len(result.Layers)).To(Equal(1))

// 				layer := result.Layers[0]
// 				Expect(layer.Name).To(Equal("build-modules"))
// 				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "build-modules")))
// 				Expect(layer.BuildEnv).To(Equal(packit.Environment{
// 					"PATH.append":       filepath.Join(layersDir, "build-modules", "node_modules", ".bin"),
// 					"PATH.delim":        ":",
// 					"NODE_ENV.override": "development",
// 				}))
// 				Expect(layer.Build).To(BeTrue())
// 				Expect(layer.Cache).To(BeTrue())
// 				Expect(layer.Metadata).To(Equal(
// 					map[string]interface{}{
// 						"cache_sha": "some-awesome-shasum",
// 					}))
// 				Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
// 					{
// 						Extension: "cdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
// 					},
// 					{
// 						Extension: "spdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
// 					},
// 					{
// 						Extension: "syft.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
// 					},
// 				}))
// 				Expect(len(layer.ExecD)).To(Equal(0))

// 				Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

// 				Expect(symlinker.LinkCall.CallCount).To(BeZero())

// 				Expect(berryInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

// 				Expect(berryInstallProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(workingDir))
// 				Expect(berryInstallProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
// 				Expect(berryInstallProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

// 				Expect(berryInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
// 				Expect(berryInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "build-modules")))
// 				Expect(berryInstallProcess.ExecuteCall.Receives.Launch).To(BeFalse())
// 			})
// 		})
// 	})

// 	context("when node_modules is required during launch", func() {
// 		it.Before(func() {
// 			entryResolver.MergeLayerTypesCall.Returns.Launch = true
// 		})

// 		context("Classic", func() {
// 			it("returns a result that installs launch modules", func() {
// 				result, err := build(packit.BuildContext{
// 					BuildpackInfo: packit.BuildpackInfo{
// 						Name:        "Some Buildpack",
// 						Version:     "1.2.3",
// 						SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 					},
// 					WorkingDir: workingDir,
// 					CNBPath:    cnbDir,
// 					Layers:     packit.Layers{Path: layersDir},
// 					Plan: packit.BuildpackPlan{
// 						Entries: []packit.BuildpackPlanEntry{
// 							{
// 								Name: "node_modules",
// 								Metadata: map[string]interface{}{
// 									"build": true,
// 								},
// 							},
// 						},
// 					},
// 					Stack: "some-stack",
// 					Platform: packit.Platform{
// 						Path: "some-platform-path",
// 					},
// 				})
// 				Expect(err).NotTo(HaveOccurred())

// 				Expect(len(result.Layers)).To(Equal(1))
// 				layer := result.Layers[0]
// 				Expect(layer.Name).To(Equal("launch-modules"))
// 				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 				Expect(layer.LaunchEnv).To(Equal(packit.Environment{
// 					"PATH.append": filepath.Join(layersDir, "launch-modules", "node_modules", ".bin"),
// 					"PATH.delim":  ":",
// 				}))
// 				Expect(layer.Launch).To(BeTrue())
// 				Expect(layer.Metadata).To(Equal(
// 					map[string]interface{}{
// 						"cache_sha": "some-awesome-shasum",
// 					}))
// 				Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
// 					{
// 						Extension: "cdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
// 					},
// 					{
// 						Extension: "spdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
// 					},
// 					{
// 						Extension: "syft.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
// 					},
// 				}))

// 				Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

// 				Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

// 				Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
// 				Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
// 				Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

// 				Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
// 				Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
// 				Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

// 				Expect(symlinker.LinkCall.CallCount).To(BeZero())

// 				Expect(classicInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

// 				Expect(classicInstallProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(workingDir))
// 				Expect(classicInstallProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
// 				Expect(classicInstallProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

// 				Expect(classicInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
// 				Expect(classicInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 				Expect(classicInstallProcess.ExecuteCall.Receives.Launch).To(BeTrue())

// 				Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
// 			})
// 		})

// 		context("Berry", func() {
// 			it.Before(func() {
// 				yarnrcYmlParser.ParseLinkerCall.Returns.Err = nil
// 			})

// 			it("returns a result that installs launch modules", func() {
// 				result, err := build(packit.BuildContext{
// 					BuildpackInfo: packit.BuildpackInfo{
// 						Name:        "Some Buildpack",
// 						Version:     "1.2.3",
// 						SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 					},
// 					WorkingDir: workingDir,
// 					CNBPath:    cnbDir,
// 					Layers:     packit.Layers{Path: layersDir},
// 					Plan: packit.BuildpackPlan{
// 						Entries: []packit.BuildpackPlanEntry{
// 							{
// 								Name: "node_modules",
// 								Metadata: map[string]interface{}{
// 									"build": true,
// 								},
// 							},
// 						},
// 					},
// 					Stack: "some-stack",
// 					Platform: packit.Platform{
// 						Path: "some-platform-path",
// 					},
// 				})
// 				Expect(err).NotTo(HaveOccurred())

// 				Expect(len(result.Layers)).To(Equal(1))
// 				layer := result.Layers[0]
// 				Expect(layer.Name).To(Equal("launch-modules"))
// 				Expect(layer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 				Expect(layer.LaunchEnv).To(Equal(packit.Environment{
// 					"PATH.append": filepath.Join(layersDir, "launch-modules", "node_modules", ".bin"),
// 					"PATH.delim":  ":",
// 				}))
// 				Expect(layer.Launch).To(BeTrue())
// 				Expect(layer.Metadata).To(Equal(
// 					map[string]interface{}{
// 						"cache_sha": "some-awesome-shasum",
// 					}))
// 				Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
// 					{
// 						Extension: "cdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
// 					},
// 					{
// 						Extension: "spdx.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
// 					},
// 					{
// 						Extension: "syft.json",
// 						Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
// 					},
// 				}))

// 				Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

// 				Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

// 				Expect(symlinker.LinkCall.CallCount).To(BeZero())

// 				Expect(berryInstallProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

// 				Expect(berryInstallProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(workingDir))
// 				Expect(berryInstallProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
// 				Expect(berryInstallProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

// 				Expect(berryInstallProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
// 				Expect(berryInstallProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 				Expect(berryInstallProcess.ExecuteCall.Receives.Launch).To(BeTrue())

// 				Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
// 			})
// 		})
// 	})

// 	context("when node_modules is required during neither build nor launch", func() {
// 		it("returns a result that has no layers", func() {
// 			result, err := build(packit.BuildContext{
// 				BuildpackInfo: packit.BuildpackInfo{
// 					Name:        "Some Buildpack",
// 					Version:     "1.2.3",
// 					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 				},
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{
// 							Name: "node_modules",
// 							Metadata: map[string]interface{}{
// 								"build": true,
// 							},
// 						},
// 					},
// 				},
// 				Stack: "some-stack",
// 				Platform: packit.Platform{
// 					Path: "some-platform-path",
// 				},
// 			})
// 			Expect(err).NotTo(HaveOccurred())
// 			Expect(result).To(Equal(packit.BuildResult{}))
// 		})
// 	})

// 	context("when node_modules is during both build or launch", func() {
// 		type setupModulesParams struct {
// 			WorkingDir              string
// 			CurrentModulesLayerPath string
// 			NextModulesLayerPath    string
// 			TempDir                 string
// 		}

// 		var setupModulesCalls []setupModulesParams

// 		it.Before(func() {
// 			entryResolver.MergeLayerTypesCall.Returns.Launch = true
// 			entryResolver.MergeLayerTypesCall.Returns.Build = true

// 			classicInstallProcess.SetupModulesCall.Stub = func(w string, c string, n string, t string) (string, error) {
// 				setupModulesCalls = append(setupModulesCalls, setupModulesParams{
// 					WorkingDir:              w,
// 					CurrentModulesLayerPath: c,
// 					NextModulesLayerPath:    n,
// 					TempDir:                 t,
// 				})
// 				return n, nil
// 			}
// 		})
// 		context("Classic", func() {
// 			it("returns a result that has both layers and the module setup updates accordingly", func() {
// 				result, err := build(packit.BuildContext{
// 					BuildpackInfo: packit.BuildpackInfo{
// 						Name:        "Some Buildpack",
// 						Version:     "1.2.3",
// 						SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 					},
// 					WorkingDir: workingDir,
// 					CNBPath:    cnbDir,
// 					Layers:     packit.Layers{Path: layersDir},
// 					Plan: packit.BuildpackPlan{
// 						Entries: []packit.BuildpackPlanEntry{
// 							{
// 								Name: "node_modules",
// 								Metadata: map[string]interface{}{
// 									"build": true,
// 								},
// 							},
// 						},
// 					},
// 					Stack: "some-stack",
// 					Platform: packit.Platform{
// 						Path: "some-platform-path",
// 					},
// 				})
// 				Expect(err).NotTo(HaveOccurred())

// 				launchLayer := result.Layers[1]
// 				Expect(launchLayer.ExecD).To(Equal([]string{filepath.Join(cnbDir, "bin", "setup-symlinks")}))
// 				Expect(len(result.Layers)).To(Equal(2))

// 				Expect(classicInstallProcess.SetupModulesCall.CallCount).To(Equal(2))

// 				Expect(setupModulesCalls[0].WorkingDir).To(Equal(workingDir))
// 				Expect(setupModulesCalls[0].CurrentModulesLayerPath).To(Equal(""))
// 				Expect(setupModulesCalls[0].NextModulesLayerPath).To(Equal(result.Layers[0].Path))

// 				Expect(setupModulesCalls[1].WorkingDir).To(Equal(workingDir))
// 				Expect(setupModulesCalls[1].CurrentModulesLayerPath).To(Equal(result.Layers[0].Path))
// 				Expect(setupModulesCalls[1].NextModulesLayerPath).To(Equal(result.Layers[1].Path))
// 			})
// 		})

// 		context("Berry", func() {
// 			it.Before(func() {
// 				yarnrcYmlParser.ParseLinkerCall.Returns.Err = nil
// 			})

// 			it("returns a result that has both layers and the module setup updates accordingly", func() {
// 				result, err := build(packit.BuildContext{
// 					BuildpackInfo: packit.BuildpackInfo{
// 						Name:        "Some Buildpack",
// 						Version:     "1.2.3",
// 						SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 					},
// 					WorkingDir: workingDir,
// 					CNBPath:    cnbDir,
// 					Layers:     packit.Layers{Path: layersDir},
// 					Plan: packit.BuildpackPlan{
// 						Entries: []packit.BuildpackPlanEntry{
// 							{
// 								Name: "node_modules",
// 								Metadata: map[string]interface{}{
// 									"build": true,
// 								},
// 							},
// 						},
// 					},
// 					Stack: "some-stack",
// 					Platform: packit.Platform{
// 						Path: "some-platform-path",
// 					},
// 				})
// 				Expect(err).NotTo(HaveOccurred())

// 				launchLayer := result.Layers[1]
// 				Expect(launchLayer.ExecD).To(Equal([]string{filepath.Join(cnbDir, "bin", "setup-symlinks")}))
// 				Expect(len(result.Layers)).To(Equal(2))

// 				Expect(berryInstallProcess.SetupModulesCall.CallCount).To(Equal(2))

// 				Expect(setupModulesCalls[0].WorkingDir).To(Equal(workingDir))
// 				Expect(setupModulesCalls[0].CurrentModulesLayerPath).To(Equal(""))
// 				Expect(setupModulesCalls[0].NextModulesLayerPath).To(Equal(result.Layers[0].Path))

// 				Expect(setupModulesCalls[1].WorkingDir).To(Equal(workingDir))
// 				Expect(setupModulesCalls[1].CurrentModulesLayerPath).To(Equal(result.Layers[0].Path))
// 				Expect(setupModulesCalls[1].NextModulesLayerPath).To(Equal(result.Layers[1].Path))
// 			})
// 		})
// 	})

// 	context("when re-using previous modules layer", func() {
// 		it.Before(func() {
// 			classicInstallProcess.ShouldRunCall.Stub = nil
// 			classicInstallProcess.ShouldRunCall.Returns.Run = false
// 			entryResolver.MergeLayerTypesCall.Returns.Launch = true
// 			entryResolver.MergeLayerTypesCall.Returns.Build = true
// 		})

// 		it("does not redo the build process", func() {
// 			result, err := build(packit.BuildContext{
// 				BuildpackInfo: packit.BuildpackInfo{
// 					Name:        "Some Buildpack",
// 					Version:     "1.2.3",
// 					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 				},
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{
// 							Name: "node_modules",
// 							Metadata: map[string]interface{}{
// 								"build": true,
// 							},
// 						},
// 					},
// 				},
// 				Stack: "some-stack",
// 				Platform: packit.Platform{
// 					Path: "some-platform-path",
// 				},
// 			})
// 			Expect(err).NotTo(HaveOccurred())

// 			Expect(len(result.Layers)).To(Equal(2))
// 			buildLayer := result.Layers[0]
// 			Expect(buildLayer.Name).To(Equal("build-modules"))
// 			Expect(buildLayer.Path).To(Equal(filepath.Join(layersDir, "build-modules")))
// 			Expect(buildLayer.Build).To(BeTrue())
// 			Expect(buildLayer.Cache).To(BeTrue())

// 			launchLayer := result.Layers[1]
// 			Expect(launchLayer.Name).To(Equal("launch-modules"))
// 			Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 			Expect(launchLayer.Launch).To(BeTrue())

// 			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
// 			Expect(symlinker.LinkCall.Receives.Oldname).To(Equal(filepath.Join(layersDir, "build-modules", "node_modules")))
// 			Expect(symlinker.LinkCall.Receives.Newname).To(Equal(filepath.Join(workingDir, "some-project-dir", "node_modules")))
// 		})

// 		context("Berry", func() {
// 		})
// 	})

// 	context("when re-using previous launch modules layer", func() {
// 		it.Before(func() {
// 			classicInstallProcess.ShouldRunCall.Stub = nil
// 			classicInstallProcess.ShouldRunCall.Returns.Run = false
// 			entryResolver.MergeLayerTypesCall.Returns.Launch = true
// 		})

// 		it("does not redo the build process", func() {
// 			result, err := build(packit.BuildContext{
// 				BuildpackInfo: packit.BuildpackInfo{
// 					Name:        "Some Buildpack",
// 					Version:     "1.2.3",
// 					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 				},
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{
// 							Name: "node_modules",
// 							Metadata: map[string]interface{}{
// 								"build": true,
// 							},
// 						},
// 					},
// 				},
// 				Stack: "some-stack",
// 				Platform: packit.Platform{
// 					Path: "some-platform-path",
// 				},
// 			})
// 			Expect(err).NotTo(HaveOccurred())

// 			Expect(len(result.Layers)).To(Equal(1))
// 			launchLayer := result.Layers[0]
// 			Expect(launchLayer.Name).To(Equal("launch-modules"))
// 			Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 			Expect(launchLayer.Launch).To(BeTrue())

// 			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
// 			Expect(symlinker.LinkCall.Receives.Oldname).To(Equal(filepath.Join(layersDir, "launch-modules", "node_modules")))
// 			Expect(symlinker.LinkCall.Receives.Newname).To(Equal(filepath.Join(workingDir, "some-project-dir", "node_modules")))
// 		})

// 		context("Berry", func() {
// 		})
// 	})

// 	context("when yarn_pkgs is required", func() {
// 		it.Before(func() {
// 			yarnrcYmlParser.ParseLinkerCall.Returns.Err = nil
// 		})

// 		context("when all PnP artifacts are present (.yarn/cache, .pnp.cjs & .yarnrc.yml)", func() {
// 			it.Before(func() {
// 				entryResolver.MergeLayerTypesCall.Returns.Build = true
// 			})
// 			it("does not run yarn ", func() {
// 			})
// 		})

// 	})

// }
