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
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testCaching(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker

		imageIDs     map[string]struct{}
		containerIDs map[string]struct{}

		name   string
		source string
	)

	it.Before(func() {
		imageIDs = make(map[string]struct{})
		containerIDs = make(map[string]struct{})

		pack = occam.NewPack()
		docker = occam.NewDocker()

		var err error
		name, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		for id := range containerIDs {
			Expect(docker.Container.Remove.Execute(id)).To(Succeed())
		}

		for id := range imageIDs {
			Expect(docker.Image.Remove.Execute(id)).To(Succeed())
		}

		Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("a fixture is pushed twice", func() {
		context("online", func() {
			it("reuses the node_modules layer", func() {
				var err error
				var container occam.Container

				source, err = occam.Source(filepath.Join("testdata", "simple_app"))
				Expect(err).NotTo(HaveOccurred())

				build := pack.WithNoColor().Build.WithBuildpacks(nodeURI, yarnURI, buildpackURI, buildPlanURI)

				firstImage, firstLogs, err := build.Execute(name, source)
				Expect(err).NotTo(HaveOccurred(), firstLogs.String)

				imageIDs[firstImage.ID] = struct{}{}

				Expect(firstImage.Buildpacks).To(HaveLen(4))
				Expect(firstImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
				Expect(firstImage.Buildpacks[2].Layers).To(HaveKey("modules"))

				container, err = docker.Container.Run.
					WithCommand("ls -alR /layers/paketo-buildpacks_yarn-install/modules/node_modules").
					Execute(firstImage.ID)
				Expect(err).NotTo(HaveOccurred())

				containerIDs[container.ID] = struct{}{}

				Eventually(func() string {
					cLogs, err := docker.Container.Logs.Execute(container.ID)
					Expect(err).NotTo(HaveOccurred())
					return cLogs.String()
				}).Should(ContainSubstring("leftpad"))

				secondImage, secondLogs, err := build.Execute(name, source)
				Expect(err).NotTo(HaveOccurred(), secondLogs.String)

				imageIDs[secondImage.ID] = struct{}{}

				Expect(secondImage.Buildpacks).To(HaveLen(4))
				Expect(secondImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
				Expect(secondImage.Buildpacks[2].Layers).To(HaveKey("modules"))

				container, err = docker.Container.Run.
					WithCommand("ls -alR /layers/paketo-buildpacks_yarn-install/modules/node_modules").
					Execute(secondImage.ID)
				Expect(err).NotTo(HaveOccurred())

				containerIDs[container.ID] = struct{}{}

				Eventually(func() string {
					cLogs, err := docker.Container.Logs.Execute(container.ID)
					Expect(err).NotTo(HaveOccurred())
					return cLogs.String()
				}).Should(ContainSubstring("leftpad"))

				Expect(secondImage.Buildpacks[2].Layers["modules"].Metadata["built_at"]).To(Equal(firstImage.Buildpacks[2].Layers["modules"].Metadata["built_at"]))
				Expect(secondImage.Buildpacks[2].Layers["modules"].Metadata["cache_sha"]).To(Equal(firstImage.Buildpacks[2].Layers["modules"].Metadata["cache_sha"]))

				Expect(secondImage.ID).To(Equal(firstImage.ID), fmt.Sprintf("%s\n\n%s", firstLogs, secondLogs))

				Expect(secondLogs).To(ContainLines(
					fmt.Sprintf("%s %s", buildpackInfo.Buildpack.Name, "1.2.3"),
					"  Resolving installation process",
					"    Process inputs:",
					"      yarn.lock -> Found",
					"",
					"    Selected default build process: 'yarn install'",
					"",
					fmt.Sprintf("  Reusing cached layer /layers/%s/modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				))
			})
		})
	})
}
