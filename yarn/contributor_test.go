package yarn_test

import (
	"github.com/cloudfoundry/libcfbuildpack/buildpackplan"
	"github.com/sclevine/spec/report"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

func TestUnitContributor(t *testing.T) {
	spec.Run(t, "Contributor", testContributor, spec.Report(report.Terminal{}))
}

func testContributor(t *testing.T, when spec.G, it spec.S) {
	var f *test.BuildFactory
	it.Before(func() {
		RegisterTestingT(t)
		f = test.NewBuildFactory(t)

	})

	when("NewContributor", func() {
		it("returns true if the dep is in the build plan", func() {
			f.AddPlan(buildpackplan.Plan{Name: yarn.Dependency})

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
			f.AddPlan(buildpackplan.Plan{Name: yarn.Dependency})
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
			f.AddPlan(buildpackplan.Plan{
				Name: yarn.Dependency,
				Metadata: buildpackplan.Metadata{"build": true, "launch": true},
			})
			f.AddDependency(yarn.Dependency, stubYarnFixture)

			yarnDep, _, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())

			Expect(yarnDep.Contribute()).To(Succeed())

			layer := f.Build.Layers.Layer(yarn.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(true, true, true))
		})
	})
}
