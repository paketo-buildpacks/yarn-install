package yarninstall_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
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
		buildLayerBuilder    *fakes.LayerBuilder
		launchLayerBuilder   *fakes.LayerBuilder
		entryResolver        *fakes.EntryResolver
		linkCalls            []linkCallParams
		pathParser           *fakes.PathParser
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

		// Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		// Expect(os.MkdirAll(filepath.Join(cnbDir, "bin"), os.ModePerm)).To(Succeed())
		// Expect(os.WriteFile(filepath.Join(cnbDir, "bin", "setup-symlinks"), []byte(""), os.ModePerm)).To(Succeed())

		entryResolver = &fakes.EntryResolver{}

		buffer = bytes.NewBuffer(nil)

		pathParser = &fakes.PathParser{}
		pathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "some-project-dir")

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

		buildLayerBuilder = &fakes.LayerBuilder{}
		buildLayerBuilder.BuildCall.Returns.Layer = packit.Layer{Name: "build-modules"}

		launchLayerBuilder = &fakes.LayerBuilder{}
		launchLayerBuilder.BuildCall.Returns.Layer = packit.Layer{Name: "launch-modules"}

		build = yarninstall.Build(
			pathParser,
			entryResolver,
			configurationManager,
			homeDir,
			symlinker,
			buildLayerBuilder,
			launchLayerBuilder,
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

			Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

			Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

			Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
			Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

			Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
			Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

			Expect(buildLayerBuilder.BuildCall.Receives.CurrentModulesLayerPath).To(Equal(""))
			Expect(buildLayerBuilder.BuildCall.Receives.ProjectPath).To(Equal(filepath.Join(workingDir, "some-project-dir")))

			Expect(symlinker.LinkCall.CallCount).To(BeZero())
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

			Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

			Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

			Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
			Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

			Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
			Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

			Expect(launchLayerBuilder.BuildCall.Receives.CurrentModulesLayerPath).To(Equal(""))
			Expect(launchLayerBuilder.BuildCall.Receives.ProjectPath).To(Equal(filepath.Join(workingDir, "some-project-dir")))

			Expect(symlinker.LinkCall.CallCount).To(BeZero())

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

	context("when required during both build and launch", func() {
		it.Before(func() {
			buildLayerBuilder.BuildCall.Returns.Layer.Path = "existing-path"
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
			entryResolver.MergeLayerTypesCall.Returns.Build = true
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

			Expect(buildLayerBuilder.BuildCall.Receives.CurrentModulesLayerPath).To(Equal(""))
			Expect(buildLayerBuilder.BuildCall.Receives.ProjectPath).To(Equal(filepath.Join(workingDir, "some-project-dir")))

			Expect(launchLayerBuilder.BuildCall.Receives.CurrentModulesLayerPath).To(Equal("existing-path"))
			Expect(launchLayerBuilder.BuildCall.Receives.ProjectPath).To(Equal(filepath.Join(workingDir, "some-project-dir")))
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
