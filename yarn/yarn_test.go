package yarn

import (
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/sclevine/spec/report"

	"github.com/cloudfoundry/libcfbuildpack/test"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

func TestUnitYarn(t *testing.T) {
	RegisterTestingT(t)
	spec.Run(t, "Yarn", testYarn, spec.Report(report.Terminal{}))
}

func testYarn(t *testing.T, when spec.G, it spec.S) {
	when("NewContributor", func() {
		var stubYarnFixture = filepath.Join("stub-yarn.tar.gz")

		it("returns true if a build plan exists", func() {
			f := test.NewBuildFactory(t)
			f.AddBuildPlan(t, Dependency, buildplan.Dependency{})
			f.AddDependency(t, Dependency, stubYarnFixture)

			_, willContribute, err := NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())
			Expect(willContribute).To(BeTrue())
		})

		it("returns false if a build plan does not exist", func() {
			f := test.NewBuildFactory(t)

			_, willContribute, err := NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())
			Expect(willContribute).To(BeFalse())
		})

		it("contributes yarn to the cache layer when included in the build plan", func() {
			f := test.NewBuildFactory(t)
			f.AddBuildPlan(t, Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"build": true},
			})
			f.AddDependency(t, Dependency, stubYarnFixture)

			yarnDep, _, err := NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())

			Expect(yarnDep.Contribute()).To(Succeed())

			layer := f.Build.Layers.Layer(Dependency)
			test.BeLayerLike(t, layer, true, true, false)
			test.BeFileLike(t, filepath.Join(layer.Root, "stub.txt"), 0644, "This is a stub file\n")
		})

		it("contributes yarn to the launch layer when included in the build plan", func() {
			f := test.NewBuildFactory(t)
			f.AddBuildPlan(t, Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"launch": true},
			})
			f.AddDependency(t, Dependency, stubYarnFixture)

			yarnContributor, _, err := NewContributor(f.Build)
			Expect(err).NotTo(HaveOccurred())

			Expect(yarnContributor.Contribute()).To(Succeed())

			layer := f.Build.Layers.Layer(Dependency)
			test.BeLayerLike(t, layer, false, true, true)
			test.BeFileLike(t, filepath.Join(layer.Root, "stub.txt"), 0644, "This is a stub file\n")
		})
	})
}
