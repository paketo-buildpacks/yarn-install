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

func testNoHoist(t *testing.T, context spec.G, it spec.S) {
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

	context("when using yarn workspaces with nohoist", func() {
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

			source, err = occam.Source(filepath.Join("testdata", "with_workspaces_nohoist"))
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

		it("should correctly run the app", func() {
			var err error
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

		it("should correctly install node modules without hoisting", func() {
			var err error
			image, _, err = pack.Build.
				WithExtensions(
					settings.Extensions.UbiNodejsExtension.Online,
				).
				WithPullPolicy(pullPolicy).
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithPullPolicy(pullPolicy).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			expressModuleHoistPath := "/workspace/node_modules/express"
			container, err = docker.Container.Run.
				WithEntrypoint("stat").
				WithCommand(expressModuleHoistPath).
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				logs, _ := docker.Container.Logs.Execute(container.ID)
				return logs.String()
			}).Should(ContainSubstring("No such file or directory"))
		})
	})
}
