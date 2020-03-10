package integration_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testLogging(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when app is NOT vendored", func() {
		var (
			image occam.Image

			name string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		})

		it("should build a working OCI image for a simple app", func() {
			var err error
			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithBuildpacks(nodeURI, yarnURI).
				WithNoPull().
				Execute(name, filepath.Join("testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())

			buildpackVersion, err := GetGitVersion()
			Expect(err).ToNot(HaveOccurred())

			splitLogs := GetBuildLogs(logs.String())
			Expect(splitLogs).To(ContainSequence([]interface{}{
				fmt.Sprintf("Yarn Install Buildpack %s", buildpackVersion),
				"  Executing build process",
				MatchRegexp(`    Installing Yarn 1\.\d+\.\d+`),
				MatchRegexp(`      Completed in (\d+\.\d+|\d{3})`),
				"",
				"  Resolving installation process",
				"    Process inputs:",
				"      yarn.lock -> Found",
				"",
				"    Selected default build process: 'yarn install'",
				"",
				"  Executing build process",
				"    Running yarn install --ignore-engines --frozen-lockfile --modules-folder /layers/org.cloudfoundry.yarn-install/modules/node_modules",
				MatchRegexp(`      Completed in (\d+\.\d+|\d{3})`),
			},
			), logs.String)
		})
	})

	context("when the app is vendored", func() {
		var (
			image occam.Image

			name string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
		})

		it("should build a working OCI image for a simple app", func() {
			var err error
			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithBuildpacks(nodeURI, yarnURI).
				WithNoPull().
				Execute(name, filepath.Join("testdata", "vendored"))
			Expect(err).NotTo(HaveOccurred())

			buildpackVersion, err := GetGitVersion()
			Expect(err).ToNot(HaveOccurred())

			splitLogs := GetBuildLogs(logs.String())
			Expect(splitLogs).To(ContainSequence([]interface{}{
				fmt.Sprintf("Yarn Install Buildpack %s", buildpackVersion),
				"  Executing build process",
				MatchRegexp(`    Installing Yarn 1\.\d+\.\d+`),
				MatchRegexp(`      Completed in (\d+\.\d+|\d{3})`),
				"",
				"  Resolving installation process",
				"    Process inputs:",
				"      yarn.lock -> Found",
				"",
				"    Selected default build process: 'yarn install'",
				"",
				"  Executing build process",
				"    Running yarn install --ignore-engines --frozen-lockfile --offline --modules-folder /layers/org.cloudfoundry.yarn-install/modules/node_modules",
				MatchRegexp(`      Completed in (\d+\.\d+|\d{3})`),
			},
			), logs.String)
		})
	})
}
