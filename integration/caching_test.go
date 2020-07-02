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
				source, err = occam.Source(filepath.Join("testdata", "simple_app"))
				Expect(err).NotTo(HaveOccurred())

				build := pack.WithNoColor().Build.WithBuildpacks(nodeURI, yarnURI)

				firstImage, firstLogs, err := build.Execute(name, source)
				Expect(err).NotTo(HaveOccurred(), firstLogs.String)

				imageIDs[firstImage.ID] = struct{}{}

				Expect(firstImage.Buildpacks).To(HaveLen(2))
				Expect(firstImage.Buildpacks[1].Key).To(Equal(buildpackInfo.Buildpack.ID))
				Expect(firstImage.Buildpacks[1].Layers).To(HaveKey("yarn"))
				Expect(firstImage.Buildpacks[1].Layers).To(HaveKey("modules"))

				container, err := docker.Container.Run.Execute(firstImage.ID)
				Expect(err).NotTo(HaveOccurred())

				containerIDs[container.ID] = struct{}{}

				Eventually(container).Should(BeAvailable())

				secondImage, secondLogs, err := build.Execute(name, source)
				Expect(err).NotTo(HaveOccurred(), secondLogs.String)

				imageIDs[secondImage.ID] = struct{}{}

				Expect(secondImage.Buildpacks).To(HaveLen(2))
				Expect(secondImage.Buildpacks[1].Key).To(Equal(buildpackInfo.Buildpack.ID))
				Expect(secondImage.Buildpacks[1].Layers).To(HaveKey("yarn"))
				Expect(secondImage.Buildpacks[1].Layers).To(HaveKey("modules"))

				container, err = docker.Container.Run.Execute(secondImage.ID)
				Expect(err).NotTo(HaveOccurred())

				containerIDs[container.ID] = struct{}{}

				Eventually(container).Should(BeAvailable())

				Expect(secondImage.Buildpacks[1].Layers["yarn"].Metadata["built_at"]).To(Equal(firstImage.Buildpacks[1].Layers["yarn"].Metadata["built_at"]))
				Expect(secondImage.Buildpacks[1].Layers["yarn"].Metadata["cache_sha"]).To(Equal(firstImage.Buildpacks[1].Layers["yarn"].Metadata["cache_sha"]))

				Expect(secondImage.Buildpacks[1].Layers["modules"].Metadata["built_at"]).To(Equal(firstImage.Buildpacks[1].Layers["modules"].Metadata["built_at"]))
				Expect(secondImage.Buildpacks[1].Layers["modules"].Metadata["cache_sha"]).To(Equal(firstImage.Buildpacks[1].Layers["modules"].Metadata["cache_sha"]))

				Expect(secondImage.ID).To(Equal(firstImage.ID), fmt.Sprintf("%s\n\n%s", firstLogs, secondLogs))

				buildpackVersion, err := GetGitVersion()
				Expect(err).ToNot(HaveOccurred())

				Expect(secondLogs).To(ContainLines(
					fmt.Sprintf("%s %s", buildpackInfo.Buildpack.Name, buildpackVersion),
					fmt.Sprintf("  Reusing cached layer /layers/%s/yarn", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
					"",
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
