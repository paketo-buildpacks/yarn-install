package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testModuleBinaries(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when the node_modules contain an executable in .bin", func() {
		var (
			image     occam.Image
			container occam.Container

			name   string
			source string

			pullPolicy = "never"
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

		it("builds a working OCI image for a vendored app with binary", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "module_binaries"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			image, logs, err = pack.WithVerbose().WithNoColor().Build.
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
			Expect(err).NotTo(HaveOccurred(), logs.String)

			container, err = docker.Container.Run.WithTTY().
				WithCommand(`chalk bold 'PAKETO' && yarn start`).
				Execute(image.ID)
			Expect(err).NotTo(HaveOccurred())
		})
	})
}
