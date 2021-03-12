package yarninstall_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/paketo-buildpacks/packit/scribe"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir  string
		workingDir string
		cnbDir     string
		timestamp  string

		buffer            *bytes.Buffer
		cacheMatcher      *fakes.CacheMatcher
		clock             chronos.Clock
		dependencyService *fakes.DependencyService
		installProcess    *fakes.InstallProcess
		now               time.Time
		pathParser        *fakes.PathParser

		build packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = ioutil.TempDir("", "layers")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = ioutil.TempDir("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())

		cnbDir, err = ioutil.TempDir("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		now = time.Now()
		clock = chronos.NewClock(func() time.Time {
			return now
		})

		timestamp = now.Format(time.RFC3339Nano)

		installProcess = &fakes.InstallProcess{}
		installProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
			return true, "some-awesome-shasum", nil
		}

		dependencyService = &fakes.DependencyService{}
		dependencyService.ResolveCall.Returns.Dependency = postal.Dependency{
			ID:           "yarn",
			Name:         "Yarn",
			SHA256:       "some-sha",
			Source:       "some-source",
			SourceSHA256: "some-source-sha",
			Stacks:       []string{"some-stack"},
			URI:          "some-uri",
			Version:      "some-version",
		}

		cacheMatcher = &fakes.CacheMatcher{}

		buffer = bytes.NewBuffer(nil)

		pathParser = &fakes.PathParser{}
		pathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "some-project-dir")

		build = yarninstall.Build(pathParser, dependencyService, cacheMatcher, installProcess, clock, scribe.NewLogger(buffer))
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	context("when adding modules layer to image", func() {
		it("resolves and calls the build process", func() {
			result, err := build(packit.BuildContext{
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
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
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
				Layers: []packit.Layer{
					{
						Name: "modules",
						Path: filepath.Join(layersDir, "modules"),
						SharedEnv: packit.Environment{
							"PATH.append": filepath.Join(layersDir, "modules", "node_modules", ".bin"),
							"PATH.delim":  ":",
						},
						BuildEnv:         packit.Environment{},
						LaunchEnv:        packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            true,
						Cache:            true,
						Metadata: map[string]interface{}{
							"built_at":  timestamp,
							"cache_sha": "some-awesome-shasum",
						},
					},
				},
			}))

			Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))
			Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "modules")))
		})
	})

	context("when re-using previous modules layer", func() {
		it.Before(func() {
			installProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
				return false, "", nil
			}
		})

		it("does not redo the build process", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Layers:     packit.Layers{Path: layersDir},
				Stack:      "some-stack",
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
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
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
				Layers: []packit.Layer{
					{
						Name:             "modules",
						Path:             filepath.Join(layersDir, "modules"),
						SharedEnv:        packit.Environment{},
						BuildEnv:         packit.Environment{},
						LaunchEnv:        packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            true,
						Cache:            true,
					},
				},
			}))
			dest, err := os.Readlink(filepath.Join(workingDir, "some-project-dir", "node_modules"))
			Expect(err).NotTo(HaveOccurred())

			Expect(dest).To(Equal(filepath.Join(layersDir, "modules", "node_modules")))
		})
	})

	context("failure cases", func() {
		context("when the modules layer cannot be retrieved", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(layersDir, "modules.toml"), nil, 0000)).To(Succeed())
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

		context("when the layers directory cannot be written to", func() {
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
	})
}
