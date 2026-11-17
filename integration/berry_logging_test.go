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

func testBerryLogging(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		pack   occam.Pack
		docker occam.Docker

		pullPolicy              = "never"
		extenderBuildStr        = ""
		extenderBuildStrEscaped = ""
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()

		if settings.Extensions.UbiNodejsExtension.Online != "" {
			pullPolicy = "always"
			extenderBuildStr = "[extender (build)] "
			extenderBuildStrEscaped = `\[extender \(build\)\] `
		}
	})

	context("when building a Yarn Berry app", func() {
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

		it("logs useful information for the Berry install process", func() {
			var err error
			source, err = occam.Source(filepath.Join("testdata", "berry_simple_app"))
			Expect(err).NotTo(HaveOccurred())

			var logs fmt.Stringer
			image, logs, err = pack.WithNoColor().Build.
				WithExtensions(
					settings.Extensions.UbiNodejsExtension.Online,
				).
				WithBuildpacks(
					nodeURI,
					yarnURI,
					buildpackURI,
					buildPlanURI,
				).
				WithEnv(map[string]string{"BP_LOG_LEVEL": "DEBUG"}).
				WithPullPolicy(pullPolicy).
				Execute(name, source)
			Expect(err).NotTo(HaveOccurred())

			Expect(logs).To(ContainLines(
				fmt.Sprintf("%s%s %s", extenderBuildStr, buildpackInfo.Buildpack.Name, "1.2.3"),
				extenderBuildStr+"  Resolving installation process",
				extenderBuildStr+"    Process inputs:",
				extenderBuildStr+"      yarn.lock -> Found",
				extenderBuildStr+"",
				extenderBuildStr+"    Selected default build process: 'yarn install'",
				extenderBuildStr+"",
				extenderBuildStr+"  Executing launch environment install process",
				extenderBuildStr+"    Running 'node /workspace/.yarn/releases/yarn-4.12.0.cjs install --immutable' (app-provided yarnPath)",
			))

			// The launch environment exposes the app entrypoint and the Berry
			// install-state path so caching works across builds.
			Expect(logs.String()).To(ContainSubstring(`NODE_PROJECT_PATH`))
			Expect(logs.String()).To(ContainSubstring(`"/workspace"`))
			Expect(logs.String()).To(ContainSubstring(`YARN_INSTALL_STATE_PATH`))
			Expect(logs.String()).To(ContainSubstring(fmt.Sprintf("/layers/%s/launch-modules/.yarn/install-state.gz", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))))

			Expect(logs).To(ContainLines(
				fmt.Sprintf(`%s  Generating SBOM for /layers/%s/launch-modules`, extenderBuildStr, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
				MatchRegexp(extenderBuildStrEscaped+`      Completed in (\d+)(\.\d+)?(ms|s)`),
				extenderBuildStr+"",
				extenderBuildStr+"  Writing SBOM in the following format(s):",
				extenderBuildStr+"    application/vnd.cyclonedx+json",
				extenderBuildStr+"    application/spdx+json",
				extenderBuildStr+"    application/vnd.syft+json",
			))
		})
	})
}
