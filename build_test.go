package yarninstall_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"

	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	type determinePathCallParams struct {
		Typ         string
		PlatformDir string
		Entry       string
	}

	type linkCallParams struct {
		Oldname string
		Newname string
	}

	var (
		Expect = NewWithT(t).Expect

		layersDir  string
		workingDir string
		homeDir    string
		cnbDir     string

		determinePathCalls   []determinePathCallParams
		configurationManager *fakes.ConfigurationManager
		buffer               *bytes.Buffer
		entryResolver        *fakes.EntryResolver
		installProcess       *fakes.InstallProcess
		linkCalls            []linkCallParams
		pathParser           *fakes.PathParser
		sbomGenerator        *fakes.SBOMGenerator
		symlinker            *fakes.SymlinkManager
		unlinkPaths          []string
		build                packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		homeDir, err = os.MkdirTemp("", "home-dir")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		installProcess = &fakes.InstallProcess{}
		installProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
			return true, "some-awesome-shasum", nil
		}

		entryResolver = &fakes.EntryResolver{}

		buffer = bytes.NewBuffer(nil)

		pathParser = &fakes.PathParser{}
		pathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "some-project-dir")

		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		configurationManager = &fakes.ConfigurationManager{}

		configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
			determinePathCalls = append(determinePathCalls, determinePathCallParams{
				Typ:         typ,
				Entry:       entry,
				PlatformDir: platform,
			})
			return "", nil
		}
		symlinker = &fakes.SymlinkManager{}
		symlinker.LinkCall.Stub = func(o, n string) error {
			linkCalls = append(linkCalls, linkCallParams{
				Oldname: o,
				Newname: n,
			})
			return nil
		}
		symlinker.UnlinkCall.Stub = func(p string) error {
			unlinkPaths = append(unlinkPaths, p)
			return nil
		}

		build = yarninstall.Build(
			pathParser,
			entryResolver,
			configurationManager,
			homeDir,
			symlinker,
			installProcess,
			sbomGenerator,
			chronos.DefaultClock,
			scribe.NewEmitter(buffer),
		)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	context("when required during build", func() {
		it.Before(func() {
			entryResolver.MergeLayerTypesCall.Returns.Build = true
		})

		it("returns a result that installs build modules", func() {
			result, err := build(packit.BuildContext{
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
			})
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

			Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

			Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

			Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
			Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

			Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
			Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

			Expect(symlinker.LinkCall.CallCount).To(BeZero())

			Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

			Expect(installProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(workingDir))
			Expect(installProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
			Expect(installProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "build-modules")))
			Expect(installProcess.ExecuteCall.Receives.Launch).To(BeFalse())

			Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
		})
	})

	context("when required during launch", func() {
		it.Before(func() {
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
		})

		it("returns a result that installs launch modules", func() {
			result, err := build(packit.BuildContext{
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
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(result.Layers)).To(Equal(1))
			layer := result.Layers[0]
			Expect(layer.Name).To(Equal("launch-modules"))
			Expect(layer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
			Expect(layer.LaunchEnv).To(Equal(packit.Environment{
				"PATH.append": filepath.Join(layersDir, "launch-modules", "node_modules", ".bin"),
				"PATH.delim":  ":",
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
			Expect(layer.ExecD).To(Equal([]string{filepath.Join(cnbDir, "bin", "setup-symlinks")}))

			Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

			Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

			Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
			Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

			Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
			Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

			Expect(symlinker.LinkCall.CallCount).To(BeZero())

			Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

			Expect(installProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(workingDir))
			Expect(installProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
			Expect(installProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "launch-modules")))
			Expect(installProcess.ExecuteCall.Receives.Launch).To(BeTrue())

			Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
		})
	})

	context("when not required during either build or launch", func() {
		it("returns a result that has no layers", func() {
			result, err := build(packit.BuildContext{
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
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{}))
		})
	})

	context("when required during both build or launch", func() {
		type setupModulesParams struct {
			WorkingDir              string
			CurrentModulesLayerPath string
			NextModulesLayerPath    string
		}

		var setupModulesCalls []setupModulesParams

		it.Before(func() {
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
			entryResolver.MergeLayerTypesCall.Returns.Build = true

			installProcess.SetupModulesCall.Stub = func(w string, c string, n string) (string, error) {
				setupModulesCalls = append(setupModulesCalls, setupModulesParams{
					WorkingDir:              w,
					CurrentModulesLayerPath: c,
					NextModulesLayerPath:    n,
				})
				return n, nil
			}
		})
		it("returns a result that has both layers and the module setup updates accordingly", func() {
			result, err := build(packit.BuildContext{
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
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(result.Layers)).To(Equal(2))

			Expect(installProcess.SetupModulesCall.CallCount).To(Equal(2))

			Expect(setupModulesCalls[0].WorkingDir).To(Equal(workingDir))
			Expect(setupModulesCalls[0].CurrentModulesLayerPath).To(Equal(""))
			Expect(setupModulesCalls[0].NextModulesLayerPath).To(Equal(result.Layers[0].Path))

			Expect(setupModulesCalls[1].WorkingDir).To(Equal(workingDir))
			Expect(setupModulesCalls[1].CurrentModulesLayerPath).To(Equal(result.Layers[0].Path))
			Expect(setupModulesCalls[1].NextModulesLayerPath).To(Equal(result.Layers[1].Path))
		})
	})

	context("when re-using previous modules layer", func() {
		it.Before(func() {
			installProcess.ShouldRunCall.Stub = nil
			installProcess.ShouldRunCall.Returns.Run = false
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
			entryResolver.MergeLayerTypesCall.Returns.Build = true
		})

		it("does not redo the build process", func() {
			result, err := build(packit.BuildContext{
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
			})
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

			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
			Expect(symlinker.LinkCall.Receives.Oldname).To(Equal(filepath.Join(layersDir, "build-modules", "node_modules")))
			Expect(symlinker.LinkCall.Receives.Newname).To(Equal(filepath.Join(workingDir, "some-project-dir", "node_modules")))
		})
	})

	context("when re-using previous launch modules layer", func() {
		it.Before(func() {
			installProcess.ShouldRunCall.Stub = nil
			installProcess.ShouldRunCall.Returns.Run = false
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
		})

		it("does not redo the build process", func() {
			result, err := build(packit.BuildContext{
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
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(result.Layers)).To(Equal(1))
			launchLayer := result.Layers[0]
			Expect(launchLayer.Name).To(Equal("launch-modules"))
			Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
			Expect(launchLayer.Launch).To(BeTrue())

			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
			Expect(symlinker.LinkCall.Receives.Oldname).To(Equal(filepath.Join(layersDir, "launch-modules", "node_modules")))
			Expect(symlinker.LinkCall.Receives.Newname).To(Equal(filepath.Join(workingDir, "some-project-dir", "node_modules")))
		})
	})

	context("failure cases", func() {

		context("when the path parser returns an error", func() {
			it.Before(func() {
				pathParser.GetCall.Returns.Err = errors.New("path-parser-error")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("path-parser-error"))
			})
		})

		context("when determining the path for the npmrc fails", func() {
			it.Before(func() {
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "npmrc" {
						return "", errors.New("failed to determine path for npmrc")
					}
					return "", nil
				}
			})

			it("errors", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("failed to determine path for npmrc"))
			})
		})

		context("when determining the path for the yarnrc fails", func() {
			it.Before(func() {
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "yarnrc" {
						return "", errors.New("failed to determine path for yarnrc")
					}
					return "", nil
				}
			})

			it("errors", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("failed to determine path for yarnrc"))
			})
		})

		context("when .npmrc service binding symlink cannot be created", func() {
			it.Before(func() {
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "npmrc" {
						return "some-path/.npmrc", nil
					}
					return "", nil
				}

				symlinker.LinkCall.Stub = func(o string, n string) error {
					if strings.Contains(o, ".npmrc") {
						return errors.New("symlinking .npmrc error")
					}
					return nil
				}
			})

			it("errors", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("symlinking .npmrc error")))
			})
		})

		context("when .yarnrc service binding symlink cannot be created", func() {
			it.Before(func() {
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "yarnrc" {
						return "some-path/.yarnrc", nil
					}
					return "", nil
				}

				symlinker.LinkCall.Stub = func(o string, n string) error {
					if strings.Contains(o, ".yarnrc") {
						return errors.New("symlinking .yarnrc error")
					}
					return nil
				}
			})

			it("errors", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("symlinking .yarnrc error")))
			})
		})

		context("during the build installation process", func() {
			it.Before(func() {
				entryResolver.MergeLayerTypesCall.Returns.Build = true
			})
			context("when the layer cannot be retrieved", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(layersDir, "build-modules.toml"), nil, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to parse layer content metadata:")))
					Expect(err).To(MatchError(ContainSubstring("modules.toml")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the check for the install process fails", func() {
				it.Before(func() {
					installProcess.ShouldRunCall.Stub = nil
					installProcess.ShouldRunCall.Returns.Err = errors.New("failed to determine if process should run")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError("failed to determine if process should run"))
				})
			})

			context("when the layer cannot be reset", func() {
				it.Before(func() {
					Expect(os.Chmod(layersDir, 4444)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath: cnbDir,
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when modules cannot be set up", func() {
				it.Before(func() {
					installProcess.SetupModulesCall.Returns.Error = errors.New("failed to setup modules")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath: cnbDir,
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("failed to setup modules"))
				})
			})

			context("when the build install process cannot be executed", func() {
				it.Before(func() {
					installProcess.ExecuteCall.Returns.Error = errors.New("failed to execute install process")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError("failed to execute install process"))
				})
			})

			context("when the BOM cannot be generated", func() {
				it.Before(func() {
					sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
				})
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
						},
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{{Name: "node_modules"}},
						},
						Stack: "some-stack",
					})
					Expect(err).To(MatchError("failed to generate SBOM"))
				})
			})

			context("when the BOM cannot be formatted", func() {
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("unsupported SBOM format: 'random-format'"))
				})
			})

			context("when BP_DISABLE_SBOM is set incorrectly", func() {
				it.Before(func() {
					os.Setenv("BP_DISABLE_SBOM", "not-a-bool")
				})

				it.After(func() {
					os.Unsetenv("BP_DISABLE_SBOM")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to parse BP_DISABLE_SBOM")))
				})
			})

			context("when install is skipped and node_modules cannot be removed", func() {
				it.Before(func() {
					installProcess.ShouldRunCall.Stub = nil
					installProcess.ShouldRunCall.Returns.Run = false
					Expect(os.Chmod(filepath.Join(workingDir), 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(filepath.Join(workingDir), os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when install is skipped and symlinking node_modules fails", func() {
				it.Before(func() {
					installProcess.ShouldRunCall.Stub = nil
					installProcess.ShouldRunCall.Returns.Run = false
					symlinker.LinkCall.Stub = nil
					symlinker.LinkCall.Returns.Error = errors.New("some symlinking error")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(symlinker.LinkCall.CallCount).To(Equal(1))
					Expect(err).To(MatchError(ContainSubstring("some symlinking error")))
				})
			})
		})

		context("during the launch installation process", func() {
			it.Before(func() {
				entryResolver.MergeLayerTypesCall.Returns.Launch = true
			})
			context("when the layer cannot be retrieved", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(layersDir, "launch-modules.toml"), nil, 0000)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to parse layer content metadata:")))
					Expect(err).To(MatchError(ContainSubstring("modules.toml")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when the check for the install process fails", func() {
				it.Before(func() {
					installProcess.ShouldRunCall.Stub = nil
					installProcess.ShouldRunCall.Returns.Err = errors.New("failed to determine if process should run")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError("failed to determine if process should run"))
				})
			})

			context("when the layer cannot be reset", func() {
				it.Before(func() {
					Expect(os.Chmod(layersDir, 4444)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath: cnbDir,
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when modules cannot be set up", func() {
				it.Before(func() {
					installProcess.SetupModulesCall.Returns.Error = errors.New("failed to setup modules")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath: cnbDir,
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("failed to setup modules"))
				})
			})

			context("when the install process cannot be executed", func() {
				it.Before(func() {
					installProcess.ExecuteCall.Returns.Error = errors.New("failed to execute install process")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError("failed to execute install process"))
				})
			})

			context("when the BOM cannot be generated", func() {
				it.Before(func() {
					sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
				})
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
						},
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{{Name: "node_modules"}},
						},
						Stack: "some-stack",
					})
					Expect(err).To(MatchError("failed to generate SBOM"))
				})
			})

			context("when the BOM cannot be formatted", func() {
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("unsupported SBOM format: 'random-format'"))
				})
			})

			context("when BP_DISABLE_SBOM is set incorrectly", func() {
				it.Before(func() {
					os.Setenv("BP_DISABLE_SBOM", "not-a-bool")
				})

				it.After(func() {
					os.Unsetenv("BP_DISABLE_SBOM")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to parse BP_DISABLE_SBOM")))
				})
			})

			context("when install is skipped and node_modules cannot be removed", func() {
				it.Before(func() {
					installProcess.ShouldRunCall.Stub = nil
					installProcess.ShouldRunCall.Returns.Run = false
					Expect(os.Chmod(filepath.Join(workingDir), 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(filepath.Join(workingDir), os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("when install is skipped and symlinking node_modules fails", func() {
				it.Before(func() {
					installProcess.ShouldRunCall.Stub = nil
					installProcess.ShouldRunCall.Returns.Run = false
					symlinker.LinkCall.Stub = nil
					symlinker.LinkCall.Returns.Error = errors.New("some symlinking error")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
					})
					Expect(symlinker.LinkCall.CallCount).To(Equal(1))
					Expect(err).To(MatchError(ContainSubstring("some symlinking error")))
				})
			})
		})

		context("when .npmrc binding symlink can't be cleaned up", func() {
			it.Before(func() {
				symlinker.UnlinkCall.Stub = func(p string) error {
					if strings.Contains(p, ".npmrc") {
						return errors.New("unlinking .npmrc error")
					}
					return nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("unlinking .npmrc error"))
			})
		})

		context("when .yarnrc binding symlink can't be cleaned up", func() {
			it.Before(func() {
				symlinker.UnlinkCall.Stub = func(p string) error {
					if strings.Contains(p, ".yarnrc") {
						return errors.New("unlinking .yarnrc error")
					}
					return nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("unlinking .yarnrc error"))
			})
		})
	})
}
