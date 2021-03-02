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

func testProjectPathApp(t *testing.T, context spec.G, it spec.S) {
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

	context("when the node_modules are NOT vendored", func() {
		var (
			image  occam.Image
			image2 occam.Image

			container  occam.Container
			container2 occam.Container

			name   string
			source string
			log    fmt.Stringer
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
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("builds/rebuilds correctly", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "custom_project_path_app"))
			Expect(err).NotTo(HaveOccurred())

			image, _, err = pack.Build.
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithPullPolicy("never").
				WithEnv(map[string]string{"BP_NODE_PROJECT_PATH": "hello_world_server"}).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			container, err = docker.Container.Run.
				WithCommand(fmt.Sprintf("ls -alR /layers/%s/modules/node_modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))).
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(container.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(ContainSubstring("leftpad"))

			// rebuilding
			image2, log, err = pack.Build.
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithPullPolicy("never").
				WithEnv(map[string]string{"BP_NODE_PROJECT_PATH": "bye_server"}).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			Expect(log.String()).ToNot(
				ContainSubstring(
					fmt.Sprintf("Reusing cached layer /layers/%s/modules",
						strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"),
					),
				),
			)

			container2, err = docker.Container.Run.
				WithCommand(fmt.Sprintf("ls -alR /layers/%s/modules/node_modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))).
				Execute(image2.ID)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(container2.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(ContainSubstring("express"))
		})
	})
}
