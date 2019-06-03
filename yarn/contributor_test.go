package yarn_test

import (
	"github.com/sclevine/spec/report"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

func TestUnitContributor(t *testing.T) {
	spec.Run(t, "Contributor", testContributor, spec.Report(report.Terminal{}))
}

func testContributor(t *testing.T, when spec.G, it spec.S) {
	var (
		f               *test.BuildFactory
		stubNodeFixture = filepath.Join("testdata", "stub-yarn.tar.gz")
	)

	it.Before(func() {
		RegisterTestingT(t)
		f = test.NewBuildFactory(t)
		f.AddDependency(yarn.Dependency, stubNodeFixture)
	})

	when("NewContributor", func() {
		it("returns true if the dep is in the build plan", func() {
			f.AddBuildPlan(yarn.Dependency, buildplan.Dependency{})

			_, willContribute, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())
			Expect(willContribute).To(BeTrue())
		})

		it("returns false if the dep is not in the build plan", func() {
			f := test.NewBuildFactory(t)

			_, willContribute, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())
			Expect(willContribute).To(BeFalse())
		})
	})

	when("Contribute", func() {
		var (
			stubYarnFixture string
		)

		it.Before(func() {
			stubYarnFixture = filepath.Join("testdata", "stub-yarn.tar.gz")
		})

		it("contributes yarn when included in the build plan and sets cache true", func() {
			f.AddBuildPlan(yarn.Dependency, buildplan.Dependency{})
			f.AddDependency(yarn.Dependency, stubYarnFixture)

			yarnContributor, willContribute, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())
			Expect(willContribute).To(BeTrue())

			Expect(yarnContributor.Contribute()).To(Succeed())

			layer := f.Build.Layers.Layer(yarn.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(false, true, false))
			Expect(filepath.Join(layer.Root, "stub.txt")).To(BeARegularFile())
		})

		it("contributes yarn as build and launch layers when it is requested in the build plan", func() {
			f.AddBuildPlan(yarn.Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"build": true, "launch": true},
			})

			yarnDep, _, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())

			Expect(yarnDep.Contribute()).To(Succeed())

			layer := f.Build.Layers.Layer(yarn.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(true, true, true))
		})

		it("respects override.yml entries added by nodejs-compat-buildpack", func() {
			f.AddBuildPlan(yarn.Dependency, buildplan.Dependency{
				Version:  "9000.0.0",
				Metadata: buildplan.Metadata{
					"override": `---
name: node
version: 99.99.99
uri: https://buildpacks.cloudfoundry.org/dependencies/node/node.99.99.99-linux-x64.tgz
sha256: 062d906c87839d03b243e2821e10653c89b4c92878bfe2bf995dec231e117bfc
cf_stacks:
- cflinuxfs2
`,			},
			})

			yarnContributor, _, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())

			depLayer := yarnContributor.YarnLayer
			Expect(depLayer.Dependency.Version.Version.String()).To(Equal("99.99.99"))
			Expect(depLayer.Dependency.URI).To(Equal("https://buildpacks.cloudfoundry.org/dependencies/node/node.99.99.99-linux-x64.tgz"))
		})
	})
}
