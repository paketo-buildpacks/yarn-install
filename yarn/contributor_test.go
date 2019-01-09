package yarn_test

import (
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

func testContributor(t *testing.T, when spec.G, it spec.S) {
	when("NewContributor", func() {
		var stubYarnFixture = filepath.Join("fixtures", "stub-yarn.tar.gz")

		it("returns true if a build plan exists", func() {
			f := test.NewBuildFactory(t)
			f.AddBuildPlan(yarn.Dependency, buildplan.Dependency{})
			f.AddDependency(yarn.Dependency, stubYarnFixture)

			_, willContribute, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())
			Expect(willContribute).To(BeTrue())
		})

		it("returns false if a build plan does not exist", func() {
			f := test.NewBuildFactory(t)

			_, willContribute, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())
			Expect(willContribute).To(BeFalse())
		})

		it("contributes yarn to the cache layer when included in the build plan", func() {
			f := test.NewBuildFactory(t)
			f.AddBuildPlan(yarn.Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"build": true},
			})
			f.AddDependency(yarn.Dependency, stubYarnFixture)

			yarnDep, _, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())

			Expect(yarnDep.Contribute()).To(Succeed())

			layer := f.Build.Layers.Layer(yarn.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(true, true, false))
			Expect(filepath.Join(layer.Root, "stub.txt")).To(BeARegularFile())
		})

		it("contributes yarn to the launch layer when included in the build plan", func() {
			f := test.NewBuildFactory(t)
			f.AddBuildPlan(yarn.Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"launch": true},
			})
			f.AddDependency(yarn.Dependency, stubYarnFixture)

			yarnContributor, _, err := yarn.NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())

			Expect(yarnContributor.Contribute()).To(Succeed())

			layer := f.Build.Layers.Layer(yarn.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(false, true, true))
			Expect(filepath.Join(layer.Root, "stub.txt")).To(BeARegularFile())
		})
	})
}
