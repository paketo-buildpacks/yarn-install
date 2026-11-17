package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBerryDevDependenciesDuringBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker

		pullPolicy = "never"
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()

		if settings.Extensions.UbiNodejsExtension.Online != "" {
			pullPolicy = "always"
		}
	})

	context("when the node_modules are needed during build", func() {
		var (
			image     occam.Image
			container occam.Container

			name    string
			source  string
			sbomDir string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())

			sbomDir, err = os.MkdirTemp("", "sbom")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(sbomDir, os.ModePerm)).To(Succeed())
		})

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
			Expect(os.RemoveAll(sbomDir)).To(Succeed())
		})

		it("should build a working OCI image for an app that requires devDependencies during build", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "berry_dev_dependencies_during_build"))
			Expect(err).NotTo(HaveOccurred())

			image, _, err = pack.Build.
				WithExtensions(
					settings.Extensions.UbiNodejsExtension.Online,
				).
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithPullPolicy(pullPolicy).
				WithSBOMOutputDir(sbomDir).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			// check the contents of the launch node modules
			container, err = docker.Container.Run.
				WithCommand(fmt.Sprintf("ls -alR /layers/%s/launch-modules/node_modules",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))).
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(container.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(ContainSubstring("leftpad"))

			// check that all expected launch SBOM files are present
			Expect(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-modules", "sbom.cdx.json")).To(BeARegularFile())
			Expect(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-modules", "sbom.spdx.json")).To(BeARegularFile())
			Expect(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-modules", "sbom.syft.json")).To(BeARegularFile())

			// check that all expected build SBOM files are present
			Expect(filepath.Join(sbomDir, "sbom", "build", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "build-modules", "sbom.cdx.json")).To(BeARegularFile())
			Expect(filepath.Join(sbomDir, "sbom", "build", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "build-modules", "sbom.spdx.json")).To(BeARegularFile())
			Expect(filepath.Join(sbomDir, "sbom", "build", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "build-modules", "sbom.syft.json")).To(BeARegularFile())

			// check the launch SBOM file has an entry for an app node module
			contents, err := os.ReadFile(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-modules", "sbom.cdx.json"))
			Expect(err).NotTo(HaveOccurred())

			Expect(string(contents)).To(ContainSubstring(`"name": "leftpad"`))

			// check the build SBOM file has an entry for the devDependency installed during build
			contents, err = os.ReadFile(filepath.Join(sbomDir, "sbom", "build", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "build-modules", "sbom.cdx.json"))
			Expect(err).NotTo(HaveOccurred())

			Expect(string(contents)).To(ContainSubstring(`"name": "chalk"`))
		})
	})
}
