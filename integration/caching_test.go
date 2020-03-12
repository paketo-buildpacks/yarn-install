package integration_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/occam"
	"github.com/sclevine/spec"

	. "github.com/cloudfoundry/occam/matchers"
	. "github.com/onsi/gomega"
)

func testCaching(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack         occam.Pack
		docker       occam.Docker
		imageIDs     map[string]struct{}
		containerIDs map[string]struct{}

		imageName string
	)

	it.Before(func() {
		imageIDs = make(map[string]struct{})
		containerIDs = make(map[string]struct{})

		pack = occam.NewPack()
		docker = occam.NewDocker()

		var err error
		imageName, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		for id, _ := range containerIDs {
			Expect(docker.Container.Remove.Execute(id)).To(Succeed())
		}

		for id, _ := range imageIDs {
			Expect(docker.Image.Remove.Execute(id)).To(Succeed())
		}

		Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(imageName))).To(Succeed())
	})

	context("a fixture is pushed twice", func() {
		context("online", func() {
			it("reuses the node_modules layer", func() {
				sourcePath := filepath.Join("testdata", "simple_app")

				build := pack.WithNoColor().Build.WithBuildpacks(nodeURI, yarnURI)

				firstImage, firstLogs, err := build.Execute(imageName, sourcePath)
				Expect(err).NotTo(HaveOccurred(), firstLogs.String)

				imageIDs[firstImage.ID] = struct{}{}

				Expect(firstImage.Buildpacks).To(HaveLen(2))
				Expect(firstImage.Buildpacks[1].Key).To(Equal("org.cloudfoundry.yarn-install"))
				Expect(firstImage.Buildpacks[1].Layers).To(HaveKey("yarn"))
				Expect(firstImage.Buildpacks[1].Layers).To(HaveKey("modules"))

				container, err := docker.Container.Run.Execute(firstImage.ID)
				Expect(err).NotTo(HaveOccurred())

				containerIDs[container.ID] = struct{}{}

				Eventually(container).Should(BeAvailable(), ContainerLogs(container.ID))

				secondImage, secondLogs, err := build.Execute(imageName, sourcePath)
				Expect(err).NotTo(HaveOccurred(), secondLogs.String)

				imageIDs[secondImage.ID] = struct{}{}

				Expect(secondImage.Buildpacks).To(HaveLen(2))
				Expect(secondImage.Buildpacks[1].Key).To(Equal("org.cloudfoundry.yarn-install"))
				Expect(secondImage.Buildpacks[1].Layers).To(HaveKey("yarn"))
				Expect(secondImage.Buildpacks[1].Layers).To(HaveKey("modules"))

				container, err = docker.Container.Run.Execute(secondImage.ID)
				Expect(err).NotTo(HaveOccurred())

				containerIDs[container.ID] = struct{}{}

				Eventually(container).Should(BeAvailable(), ContainerLogs(container.ID))

				Expect(secondImage.Buildpacks[1].Layers["yarn"].Metadata["built_at"]).To(Equal(firstImage.Buildpacks[1].Layers["yarn"].Metadata["built_at"]))
				Expect(secondImage.Buildpacks[1].Layers["yarn"].Metadata["cache_sha"]).To(Equal(firstImage.Buildpacks[1].Layers["yarn"].Metadata["cache_sha"]))

				Expect(secondImage.Buildpacks[1].Layers["modules"].Metadata["built_at"]).To(Equal(firstImage.Buildpacks[1].Layers["modules"].Metadata["built_at"]))
				Expect(secondImage.Buildpacks[1].Layers["modules"].Metadata["cache_sha"]).To(Equal(firstImage.Buildpacks[1].Layers["modules"].Metadata["cache_sha"]))

				Expect(secondImage.ID).To(Equal(firstImage.ID), fmt.Sprintf("%s\n\n%s", firstLogs, secondLogs))

				buildpackVersion, err := GetGitVersion()
				Expect(err).ToNot(HaveOccurred())

				splitLogs := GetBuildLogs(secondLogs.String())
				Expect(splitLogs).To(ContainSequence([]interface{}{
					fmt.Sprintf("Yarn Install Buildpack %s", buildpackVersion),
					"  Reusing cached layer /layers/org.cloudfoundry.yarn-install/yarn",
					"",
					"  Resolving installation process",
					"    Process inputs:",
					"      yarn.lock -> Found",
					"",
					"    Selected default build process: 'yarn install'",
					"",
					"  Reusing cached layer /layers/org.cloudfoundry.yarn-install/modules",
				},
				), secondLogs.String)
			})
		})
	})
}
