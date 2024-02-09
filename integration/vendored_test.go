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

func testVendored(t *testing.T, context spec.G, it spec.S) {
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

	context("when the node_modules are vendored", func() {

		//UBI does not support offline installation at the moment,
		//so we are skipping it.
		if settings.Extensions.UbiNodejsExtension.Online != "" {
			return
		}

		var (
			image     occam.Image
			container occam.Container

			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("should build a working OCI image for a simple app", func() {
			it.After(func() {
				Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
				Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			})

			var err error
			source, err = occam.Source(filepath.Join("testdata", "vendored"))
			Expect(err).NotTo(HaveOccurred())

			image, _, err = pack.Build.
				WithBuildpacks(
					nodeOfflineURI,
					yarnOfflineURI,
					buildpackOfflineURI,
					buildPlanURI,
				).
				WithPullPolicy(pullPolicy).
				WithNetwork("none").
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			container, err = docker.Container.Run.
				WithCommand(fmt.Sprintf("ls -alR /layers/%s/launch-modules/node_modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))).
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(container.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(ContainSubstring("leftpad"))
		})

		context("and the node_modules and yarn-offline-mirror are missing dependencies required in yarn.lock", func() {
			it("should fail to build with a helpful error", func() {
				var err error
				source, err = occam.Source(filepath.Join("testdata", "vendored_with_unmet_dependencies"))
				Expect(err).NotTo(HaveOccurred())

				image, _, err = pack.Build.
					WithBuildpacks(
						nodeOfflineURI,
						yarnOfflineURI,
						buildpackOfflineURI,
						buildPlanURI,
					).
					WithNetwork("none").
					Execute(name, source)
				Expect(err).To(MatchError(ContainSubstring(`error Can't make a request in offline mode ("https://registry.yarnpkg.com/leftpad/-/leftpad-0.0.1.tgz")`)))
			})
		})
	})
}
