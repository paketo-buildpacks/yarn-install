package integration_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testWorkspaces(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack       occam.Pack
		docker     occam.Docker
		pullPolicy = "never"
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when using yarn workspaces", func() {
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

			if settings.Extensions.UbiNodejsExtension.Online != "" {
				pullPolicy = "always"
			}
		})

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		context("when offline", func() {

			//UBI does not support offline installation at the moment,
			//so we are skipping it.
			if settings.Extensions.UbiNodejsExtension.Online != "" {
				return
			}

			it("should correctly install node modules in respective workspaces", func() {
				var err error
				source, err = occam.Source(filepath.Join("testdata", "with_yarn_workspaces_offline"))
				Expect(err).NotTo(HaveOccurred())

				image, _, err = pack.Build.
					WithBuildpacks(
						nodeOfflineURI,
						yarnOfflineURI,
						buildpackOfflineURI,
						buildPlanURI,
					).
					WithNetwork("none").
					WithPullPolicy(pullPolicy).
					Execute(name, source)
				Expect(err).NotTo(HaveOccurred())

				container, err = docker.Container.Run.
					WithCommand("yarn start").
					WithEnv(map[string]string{"PORT": "8080"}).
					WithPublish("8080").
					WithPublishAll().
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())

				response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort("8080")))
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				content, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("Package A value 1"))
				Expect(string(content)).To(ContainSubstring("Package A value 2"))
			})
		})

		context("when online", func() {
			it("should correctly install node modules in respective workspaces", func() {
				var err error
				source, err = occam.Source(filepath.Join("testdata", "with_yarn_workspaces"))
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
					Execute(name, source)
				Expect(err).NotTo(HaveOccurred())

				container, err = docker.Container.Run.
					WithCommand("yarn start").
					WithEnv(map[string]string{"PORT": "8080"}).
					WithPublish("8080").
					WithPublishAll().
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())

				response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort("8080")))
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))

				content, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("Package A value 1"))
				Expect(string(content)).To(ContainSubstring("Package A value 2"))
			})
		})
	})
}
