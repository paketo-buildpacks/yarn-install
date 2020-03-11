package yarn_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/packit/postal"
	"github.com/cloudfoundry/packit/scribe"
	"github.com/cloudfoundry/yarn-install-cnb/yarn"
	"github.com/cloudfoundry/yarn-install-cnb/yarn/fakes"
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

		installProcess    *fakes.InstallProcess
		dependencyService *fakes.DependencyService
		cacheMatcher      *fakes.CacheMatcher
		clock             yarn.Clock
		now               time.Time
		buffer            *bytes.Buffer

		build packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = ioutil.TempDir("", "layers")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = ioutil.TempDir("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = ioutil.TempDir("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		now = time.Now()
		clock = yarn.NewClock(func() time.Time {
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

		build = yarn.Build(dependencyService, cacheMatcher, installProcess, clock, scribe.NewLogger(buffer))
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
						{Name: "yarn"},
					},
				},
				Stack: "some-stack",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{Name: "yarn"},
					},
				},
				Layers: []packit.Layer{
					{
						Name:      "yarn",
						Path:      filepath.Join(layersDir, "yarn"),
						SharedEnv: packit.Environment{},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     false,
						Launch:    true,
						Cache:     true,
						Metadata: map[string]interface{}{
							"built_at":  timestamp,
							"cache_sha": "some-sha",
						},
					}, {
						Name: "modules",
						Path: filepath.Join(layersDir, "modules"),
						SharedEnv: packit.Environment{
							"PATH.append": filepath.Join(layersDir, "modules", "node_modules", ".bin"),
							"PATH.delim":  ":",
						},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     false,
						Launch:    true,
						Cache:     true,
						Metadata: map[string]interface{}{
							"built_at":  timestamp,
							"cache_sha": "some-awesome-shasum",
						},
					},
				},
				Processes: []packit.Process{
					{
						Type:    "web",
						Command: "yarn start",
					},
				},
			}))

			Expect(dependencyService.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
			Expect(dependencyService.ResolveCall.Receives.Name).To(Equal("yarn"))
			Expect(dependencyService.ResolveCall.Receives.Version).To(Equal("*"))
			Expect(dependencyService.ResolveCall.Receives.Stack).To(Equal("some-stack"))

			Expect(dependencyService.InstallCall.Receives.Dependency).To(Equal(postal.Dependency{
				ID:           "yarn",
				Name:         "Yarn",
				SHA256:       "some-sha",
				Source:       "some-source",
				SourceSHA256: "some-source-sha",
				Stacks:       []string{"some-stack"},
				URI:          "some-uri",
				Version:      "some-version",
			}))
			Expect(dependencyService.InstallCall.Receives.CnbPath).To(Equal(cnbDir))
			Expect(dependencyService.InstallCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "yarn")))

			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(workingDir))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "modules")))
			Expect(installProcess.ExecuteCall.Receives.YarnLayerPath).To(Equal(filepath.Join(layersDir, "yarn")))
		})
	})

	context("when re-using previous yarn layer", func() {
		it.Before(func() {
			cacheMatcher.MatchCall.Returns.Bool = true
		})

		it("does not redo the build process", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Layers:     packit.Layers{Path: layersDir},
				Stack:      "some-stack",
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{Name: "yarn"},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{Name: "yarn"},
					},
				},
				Layers: []packit.Layer{
					{
						Name:      "yarn",
						Path:      filepath.Join(layersDir, "yarn"),
						SharedEnv: packit.Environment{},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     false,
						Launch:    true,
						Cache:     true,
					}, {
						Name: "modules",
						Path: filepath.Join(layersDir, "modules"),
						SharedEnv: packit.Environment{
							"PATH.append": filepath.Join(layersDir, "modules", "node_modules", ".bin"),
							"PATH.delim":  ":",
						},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     false,
						Launch:    true,
						Cache:     true,
						Metadata: map[string]interface{}{
							"built_at":  timestamp,
							"cache_sha": "some-awesome-shasum",
						},
					},
				},
				Processes: []packit.Process{
					{
						Type:    "web",
						Command: "yarn start",
					},
				},
			}))

			Expect(dependencyService.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
			Expect(dependencyService.ResolveCall.Receives.Name).To(Equal("yarn"))
			Expect(dependencyService.ResolveCall.Receives.Version).To(Equal("*"))
			Expect(dependencyService.ResolveCall.Receives.Stack).To(Equal("some-stack"))

			Expect(dependencyService.InstallCall.CallCount).To(Equal(0))
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
						{Name: "yarn"},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{Name: "yarn"},
					},
				},
				Layers: []packit.Layer{
					{
						Name:      "yarn",
						Path:      filepath.Join(layersDir, "yarn"),
						SharedEnv: packit.Environment{},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     false,
						Launch:    true,
						Cache:     true,
						Metadata: map[string]interface{}{
							"built_at":  timestamp,
							"cache_sha": "some-sha",
						},
					}, {
						Name:      "modules",
						Path:      filepath.Join(layersDir, "modules"),
						SharedEnv: packit.Environment{},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     false,
						Launch:    true,
						Cache:     true,
					},
				},
				Processes: []packit.Process{
					{
						Type:    "web",
						Command: "yarn start",
					},
				},
			}))

			Expect(dependencyService.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
			Expect(dependencyService.ResolveCall.Receives.Name).To(Equal("yarn"))
			Expect(dependencyService.ResolveCall.Receives.Version).To(Equal("*"))
			Expect(dependencyService.ResolveCall.Receives.Stack).To(Equal("some-stack"))

			Expect(dependencyService.InstallCall.Receives.Dependency).To(Equal(postal.Dependency{
				ID:           "yarn",
				Name:         "Yarn",
				SHA256:       "some-sha",
				Source:       "some-source",
				SourceSHA256: "some-source-sha",
				Stacks:       []string{"some-stack"},
				URI:          "some-uri",
				Version:      "some-version",
			}))
			Expect(dependencyService.InstallCall.Receives.CnbPath).To(Equal(cnbDir))
			Expect(dependencyService.InstallCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "yarn")))

			dest, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())

			Expect(dest).To(Equal(filepath.Join(layersDir, "modules", "node_modules")))
		})

	})

	context("failure cases", func() {
		context("when the yarn layer cannot be retrieved", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(layersDir, "yarn.toml"), nil, 0000)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "yarn"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("failed to parse layer content metadata:")))
				Expect(err).To(MatchError(ContainSubstring("yarn.toml")))
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when the yarn dependency fails to resolve", func() {
			it.Before(func() {
				dependencyService.ResolveCall.Returns.Error = errors.New("failed to resolve yarn")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "yarn"},
						},
					},
				})
				Expect(err).To(MatchError("failed to resolve yarn"))
			})
		})

		context("when the yarn dependency fails to install", func() {
			it.Before(func() {
				dependencyService.InstallCall.Returns.Error = errors.New("failed to install yarn")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "yarn"},
						},
					},
				})
				Expect(err).To(MatchError("failed to install yarn"))
			})
		})

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
							{Name: "yarn"},
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
							{Name: "yarn"},
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
							{Name: "yarn"},
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
							{Name: "yarn"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})
	})
}
