package yarninstall_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	var (
		Expect = NewWithT(t).Expect

		layersDir  string
		workingDir string
		cnbDir     string
		timestamp  string

		buffer         *bytes.Buffer
		clock          chronos.Clock
		installProcess *fakes.InstallProcess
		now            time.Time
		pathParser     *fakes.PathParser
		sbomGenerator  *fakes.SBOMGenerator

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

		buffer = bytes.NewBuffer(nil)

		pathParser = &fakes.PathParser{}
		pathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "some-project-dir")

		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		build = yarninstall.Build(pathParser, installProcess, clock, scribe.NewLogger(buffer), sbomGenerator)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	context("when adding modules layer to image", func() {
		it("resolves and calls the build process", func() {
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
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers[0].Name).To(Equal("modules"))
			Expect(result.Layers[0].Path).To(Equal(filepath.Join(layersDir, "modules")))
			Expect(result.Layers[0].SharedEnv).To(Equal(packit.Environment{
				"PATH.append": filepath.Join(layersDir, "modules", "node_modules", ".bin"),
				"PATH.delim":  ":",
			}))
			Expect(result.Layers[0].Build).To(BeTrue())
			Expect(result.Layers[0].Cache).To(BeTrue())
			Expect(result.Layers[0].Metadata).To(Equal(
				map[string]interface{}{
					"built_at":  timestamp,
					"cache_sha": "some-awesome-shasum",
				}))
			Expect(result.Layers[0].SBOM.Formats()).To(Equal([]packit.SBOMFormat{
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
			Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))
			Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "modules")))
			Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
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
			Expect(result.Layers[0].Name).To(Equal("modules"))
			Expect(result.Layers[0].Path).To(Equal(filepath.Join(layersDir, "modules")))
			Expect(result.Layers[0].Build).To(BeTrue())
			Expect(result.Layers[0].Cache).To(BeTrue())
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
				})
				Expect(err).To(MatchError("\"random-format\" is not a supported SBOM format"))
			})
		})
	})
}
