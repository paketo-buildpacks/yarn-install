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

func testBerrySimplePNPApp(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker

		image     occam.Image
		container occam.Container

		name    string
		source  string
		sbomDir string
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()

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

	context("when the node_modules are NOT vendored", func() {
		it("should build a working OCI image for a simple app", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "berry_simple_pnp_app"))
			Expect(err).NotTo(HaveOccurred())

			image, _, err = pack.Build.
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithPullPolicy("never").
				WithSBOMOutputDir(sbomDir).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			// check the contents of the node modules
			container, err = docker.Container.Run.
				WithCommand(fmt.Sprintf("ls -alR /layers/%s/launch-pkgs/yarn-pkgs",
					strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))).
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(container.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(ContainSubstring("leftpad"))

			// // check that all expected SBOM files are present
			// Expect(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-pkgs", "sbom.cdx.json")).To(BeARegularFile())
			// Expect(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-pkgs", "sbom.spdx.json")).To(BeARegularFile())
			// Expect(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-pkgs", "sbom.syft.json")).To(BeARegularFile())

			// // check an SBOM file to make sure it has an entry for an app node module
			// contents, err := os.ReadFile(filepath.Join(sbomDir, "sbom", "launch", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"), "launch-pkgs", "sbom.cdx.json"))
			// Expect(err).NotTo(HaveOccurred())

			// Expect(string(contents)).To(ContainSubstring(`"name": "leftpad"`))
		})
	})

}
