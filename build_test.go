package yarninstall_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
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
		tmpDir     string

		buffer                *bytes.Buffer
		yarnrcYmlParser       *fakes.YarnrcYmlParser
		pathParser            *fakes.PathParser
		berryBuild            *fakes.BuildProcess
		classicBuild          *fakes.BuildProcess
		berryInstallProcess   *fakes.InstallProcess
		classicInstallProcess *fakes.InstallProcess
		sbomGenerator         *fakes.SBOMGenerator
		entryResolver         *fakes.EntryResolver
		symlinker             *fakes.SymlinkManager
		build                 packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		tmpDir, err = os.MkdirTemp("", "tmp")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		buffer = bytes.NewBuffer(nil)

		pathParser = &fakes.PathParser{}
		pathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "some-project-dir")

		entryResolver = &fakes.EntryResolver{}

		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		yarnrcYmlParser = &fakes.YarnrcYmlParser{}
		yarnrcYmlParser.ParseLinkerCall.Returns.Err = os.ErrNotExist

		symlinker = &fakes.SymlinkManager{}

		classicBuild = &fakes.BuildProcess{}
		berryBuild = &fakes.BuildProcess{}

		build = yarninstall.Build(
			pathParser,
			yarnrcYmlParser,
			berryBuild,
			classicBuild,
			berryInstallProcess,
			classicInstallProcess,
			sbomGenerator,
			entryResolver,
			scribe.NewEmitter(buffer),
			symlinker,
			tmpDir,
		)
	})

	it("runs Yarn Classic build and returns a BuildResult", func() {
		_, err := build(packit.BuildContext{
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

		Expect(classicBuild.BuildCall.CallCount).To(Equal(1))
		Expect(berryBuild.BuildCall.CallCount).To(Equal(0))
	})

	context("when .yarnrc.yml is present", func() {
		it.Before(func() {
			yarnrcYmlParser.ParseLinkerCall.Returns.Err = nil
		})

		it("runs Yarn Berry build and returns a BuildResult", func() {
			_, err := build(packit.BuildContext{
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

			Expect(classicBuild.BuildCall.CallCount).To(Equal(0))
			Expect(berryBuild.BuildCall.CallCount).To(Equal(1))
		})
	})
}
