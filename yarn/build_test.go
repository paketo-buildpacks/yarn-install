package yarn_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	"github.com/cloudfoundry/yarn-cnb/yarn/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir           string
		workingDir          string
		cnbDir              string
		dependencyInstaller *fakes.DependencyInstaller
		installProcess      *fakes.InstallProcess

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

		err = ioutil.WriteFile(filepath.Join(cnbDir, "buildpack.toml"), []byte(`api = "0.2"
[buildpack]
  id = "org.cloudfoundry.yarn"
	name = "Yarn Buildpack"
	version = "some-version"

[metadata]
  [[metadata.dependencies]]
	  id = "yarn"
		name = "Yarn"
		sha256 = "some-sha"
		source = "some-source"
		source_sha256 = "some-source-sha"
		stacks = ["some-stack"]
		uri = "some-uri"
		version = "some-version"
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		dependencyInstaller = &fakes.DependencyInstaller{}
		installProcess = &fakes.InstallProcess{}

		build = yarn.Build(dependencyInstaller, installProcess)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

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
					Cache:     false,
				}, {
					Name:      "modules",
					Path:      filepath.Join(layersDir, "modules"),
					SharedEnv: packit.Environment{},
					BuildEnv:  packit.Environment{},
					LaunchEnv: packit.Environment{},
					Build:     false,
					Launch:    true,
					Cache:     false,
				},
			},
			Processes: []packit.Process{
				{
					Type:    "web",
					Command: "yarn start",
				},
			},
		}))

		Expect(dependencyInstaller.InstallCall.Receives.Dependencies).To(Equal([]yarn.BuildpackMetadataDependency{
			{
				ID:           "yarn",
				Name:         "Yarn",
				SHA256:       "some-sha",
				Source:       "some-source",
				SourceSHA256: "some-source-sha",
				Stacks:       []string{"some-stack"},
				URI:          "some-uri",
				Version:      "some-version",
			},
		}))
		Expect(dependencyInstaller.InstallCall.Receives.CnbPath).To(Equal(cnbDir))
		Expect(dependencyInstaller.InstallCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "yarn")))

		Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(workingDir))
		Expect(installProcess.ExecuteCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "modules")))
	})

	context("failure cases", func() {
		context("when the buildpack.toml cannot be parsed", func() {
			it.Before(func() {
				Expect(os.Remove(filepath.Join(cnbDir, "buildpack.toml"))).To(Succeed())
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
				Expect(err).To(MatchError(ContainSubstring("failed to parse buildpack.toml")))
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})

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

		context("when the yarn dependency fails to install", func() {
			it.Before(func() {
				dependencyInstaller.InstallCall.Returns.Error = errors.New("failed to install yarn")
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
	})
}
