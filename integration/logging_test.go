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

			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("should build a working OCI image for a simple app", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithEnv(map[string]string{"BP_LOG_LEVEL": "DEBUG"}).
				WithPullPolicy("never").
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			Expect(logs).To(ContainLines(
				fmt.Sprintf("%s %s", buildpackInfo.Buildpack.Name, "1.2.3"),
				"  Resolving installation process",
				"    Process inputs:",
				"      yarn.lock -> Found",
				"",
				"    Selected default build process: 'yarn install'",
				"",
				"  Executing launch environment install process",
				fmt.Sprintf("    Running yarn install --ignore-engines --frozen-lockfile --modules-folder /layers/%s/launch-modules/node_modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
				"  Configuring launch environment",
				fmt.Sprintf(`    PATH -> "$PATH:/layers/%s/launch-modules/node_modules/.bin"`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				"",
				fmt.Sprintf(`  Generating SBOM for /layers/%s/launch-modules`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
				"  Writing SBOM in the following format(s):",
				"    application/vnd.cyclonedx+json",
				"    application/spdx+json",
				"    application/vnd.syft+json",
				"",
			))
		})
	})

	context("when the app is vendored", func() {
		var (
			image occam.Image

			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("should build a working OCI image for a simple app", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "vendored"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithBuildpacks(
					nodeOfflineURI,
					yarnOfflineURI,
					buildpackOfflineURI,
					buildPlanURI,
				).
				WithNetwork("none").
				WithPullPolicy("never").
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			Expect(logs).To(ContainLines(
				fmt.Sprintf("%s %s", buildpackInfo.Buildpack.Name, "1.2.3"),
				"  Resolving installation process",
				"    Process inputs:",
				"      yarn.lock -> Found",
				"",
				"    Selected default build process: 'yarn install'",
				"",
				"  Executing launch environment install process",
				fmt.Sprintf("    Running yarn install --ignore-engines --frozen-lockfile --offline --modules-folder /layers/%s/launch-modules/node_modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
				"  Configuring launch environment",
				fmt.Sprintf(`    PATH -> "$PATH:/layers/%s/launch-modules/node_modules/.bin"`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				"",
				fmt.Sprintf(`  Generating SBOM for /layers/%s/launch-modules`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
			))
		})
	})

	context("when modules are required at build time", func() {
		var (
			image occam.Image

			name   string
			source string
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())
		})

		it.After(func() {
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("should build a working OCI image for a dev dependencies app", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "dev_dependencies_during_build"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithEnv(map[string]string{"BP_LOG_LEVEL": "DEBUG"}).
				WithPullPolicy("never").
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			Expect(logs).To(ContainLines(
				fmt.Sprintf("%s %s", buildpackInfo.Buildpack.Name, "1.2.3"),
				"  Resolving installation process",
				"    Process inputs:",
				"      yarn.lock -> Found",
				"",
				"    Selected default build process: 'yarn install'",
				"",
				"  Executing build environment install process",
				fmt.Sprintf("    Running yarn install --ignore-engines --frozen-lockfile --production false --modules-folder /layers/%s/build-modules/node_modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
				"  Configuring build environment",
				`    NODE_ENV -> "development"`,
				fmt.Sprintf(`    PATH     -> "$PATH:/layers/%s/build-modules/node_modules/.bin"`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				"",
				fmt.Sprintf(`  Generating SBOM for /layers/%s/build-modules`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
				"  Writing SBOM in the following format(s):",
				"    application/vnd.cyclonedx+json",
				"    application/spdx+json",
				"    application/vnd.syft+json",
				"",
				"  Resolving installation process",
				"    Process inputs:",
				"      yarn.lock -> Found",
				"",
				"    Selected default build process: 'yarn install'",
				"",
				"  Executing launch environment install process",
				fmt.Sprintf("    Running yarn install --ignore-engines --frozen-lockfile --modules-folder /layers/%s/launch-modules/node_modules", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
				"  Configuring launch environment",
				fmt.Sprintf(`    PATH -> "$PATH:/layers/%s/launch-modules/node_modules/.bin"`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				"",
				fmt.Sprintf(`  Generating SBOM for /layers/%s/launch-modules`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(`      Completed in (\d+)(\.\d+)?(ms|s)`),
				"",
				"  Writing SBOM in the following format(s):",
				"    application/vnd.cyclonedx+json",
				"    application/spdx+json",
				"    application/vnd.syft+json",
				"",
			))
		})
	})
}
