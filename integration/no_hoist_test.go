package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
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
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when using yarn workspaces with nohoist", func() {
		var (
			image     occam.Image
			container occam.Container

			name string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		})

		it("should correctly run the app", func() {
			var err error
			image, _, err = pack.Build.
				WithBuildpacks(nodeURI, yarnURI).
				Execute(name, filepath.Join("testdata", "with_workspaces_nohoist"))
			Expect(err).NotTo(HaveOccurred())

			container, err = docker.Container.Run.Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(container).Should(BeAvailable())
			response, err := http.Get(fmt.Sprintf("http://localhost:%s", container.HostPort()))
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			content, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("Package A value 1"))
			Expect(string(content)).To(ContainSubstring("Package A value 2"))

		})

		it("should correctly install node modules without hoisting", func() {
			var err error
			image, _, err = pack.Build.
				WithBuildpacks(nodeURI, yarnURI).
				Execute(name, filepath.Join("testdata", "with_workspaces_nohoist"))
			Expect(err).NotTo(HaveOccurred())

			expressModuleHoistPath := "/workspace/node_modules/express"
			command := fmt.Sprintf("/bin/sh -c '[ ! -d %s ]' && echo NOTEXIST", expressModuleHoistPath)

			container, err = docker.Container.Run.WithCommand(command).Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			logs, err := docker.Container.Logs.Execute(container.ID)
			Expect(err).NotTo(HaveOccurred())

			Expect(logs.String()).To(ContainSubstring("NOTEXIST"))
		})
	})
}
