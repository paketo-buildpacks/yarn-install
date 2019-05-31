package modules_test

import (
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/cloudfoundry/yarn-cnb/modules"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/yarn-cnb/yarn"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

//go:generate mockgen -source=contributor.go -destination=mocks_test.go -package=modules_test

func TestUnitModules(t *testing.T) {
	RegisterTestingT(t)
	spec.Run(t, "Modules", testModules, spec.Report(report.Terminal{}))
}

func testModules(t *testing.T, when spec.G, it spec.S) {
	const (
		cacheLayer = "modules_cache"
		dir = "node_modules"
	)

	var (
		factory *test.BuildFactory
		cacheDir = filepath.Join(".cache", "yarn")
	)

	it.Before(func() {
		RegisterTestingT(t)

		factory = test.NewBuildFactory(t)
	})

	when("NewContributor", func() {
		when("there is no yarn.lock", func() {
			it("fails", func() {
				factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{})

				_, _, err := modules.NewContributor(factory.Build, yarn.CLI{})
				Expect(err).To(HaveOccurred())
			})
		})

		when("there is a yarn.lock", func() {
			it.Before(func() {
				file := filepath.Join(factory.Build.Application.Root, "yarn.lock")
				Expect(helper.WriteFile(file, 0666, "yarn lock")).To(Succeed())
			})

			it("returns true if a build plan exists with the dep", func() {
				factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{})

				_, willContribute, err := modules.NewContributor(factory.Build, yarn.CLI{})
				Expect(err).NotTo(HaveOccurred())
				Expect(willContribute).To(BeTrue())
			})

			it("returns false if a build plan does not exist with the dep", func() {
				_, willContribute, err := modules.NewContributor(factory.Build, yarn.CLI{})
				Expect(err).NotTo(HaveOccurred())
				Expect(willContribute).To(BeFalse())
			})
		})
	})

	when("Contribute", func() {
		var (
			mockCtrl       *gomock.Controller
			mockPkgManager *MockPackageManager
			modulesLayer   layers.Layer
		)

		it.Before(func() {
			mockCtrl = gomock.NewController(t)
			mockPkgManager = NewMockPackageManager(mockCtrl)
			file := filepath.Join(factory.Build.Application.Root, "yarn.lock")
			Expect(helper.WriteFile(file, 0666, "yarn lock")).To(Succeed())

			modulesLayer = factory.Build.Layers.Layer(modules.Dependency)
			cacheLayer := factory.Build.Layers.Layer(cacheLayer)
			mockPkgManager.EXPECT().Install(
				filepath.Join(modulesLayer.Root, dir),
				filepath.Join(cacheLayer.Root, cacheDir))
			mockPkgManager.EXPECT().Check(factory.Build.Application.Root)
		})

		it.After(func() {
			mockCtrl.Finish()
		})

		it("uses yarn.lock for identity", func() {
			factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{})

			contributor, ok, err := modules.NewContributor(factory.Build, mockPkgManager)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(contributor.Contribute()).To(Succeed())
			modulesLayer := factory.Build.Layers.Layer(modules.Dependency)
			c, err := ioutil.ReadFile(modulesLayer.Metadata)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(c)).To(ContainSubstring(`Hash = "6a896d7017d636a532a914536a1cb7212c5d95a6ec5826d01e2b292e3a5d0a2a"`))
		})

		it("runs yarn install, sets env vars, and creates a symlink for node_modules", func() {
			factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"launch": true, "build": true},
			})

			contributor, ok, err := modules.NewContributor(factory.Build, mockPkgManager)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(contributor.Contribute()).To(Succeed())
			layer := factory.Build.Layers.Layer(modules.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(true, true, true))

			Expect(layer).To(test.HaveOverrideSharedEnvironment("NODE_PATH", filepath.Join(layer.Root, dir)))
			Expect(layer).To(test.HaveAppendPathSharedEnvironment("PATH", filepath.Join(layer.Root, dir, ".bin")))
			Expect(layer).To(test.HaveOverrideSharedEnvironment("npm_config_nodedir", ""))

			link, err := os.Readlink(filepath.Join(factory.Build.Application.Root, dir))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal(filepath.Join(layer.Root, dir)))

			Expect(factory.Build.Layers).To(test.HaveApplicationMetadata(layers.Metadata{Processes: []layers.Process{{"web", "yarn start"}}}))
		})

		it("contributes modules for the launch phase, cache is always true", func() {
			factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"launch": true},
			})

			contributor, ok, err := modules.NewContributor(factory.Build, mockPkgManager)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(contributor.Contribute()).To(Succeed())

			layer := factory.Build.Layers.Layer(modules.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(false, true, true))
		})

		it("contributes modules for the build phase, cache is always true", func() {
			factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{
				Metadata: buildplan.Metadata{"build": true},
			})

			contributor, ok, err := modules.NewContributor(factory.Build, mockPkgManager)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			Expect(contributor.Contribute()).To(Succeed())

			layer := factory.Build.Layers.Layer(modules.Dependency)
			Expect(layer).To(test.HaveLayerMetadata(true, true, false))
		})

		when("the app is vendored", func() {
			var (
				layerModulesDir string
			)

			it.Before(func() {
				file := filepath.Join(factory.Build.Application.Root, dir, "test_module")
				Expect(helper.WriteFile(file, 0666, "some module")).To(Succeed())

				modulesLayer := factory.Build.Layers.Layer(modules.Dependency)
				layerModulesDir = filepath.Join(modulesLayer.Root, dir)
			})

			it("moves the app node_modules to the modules layer", func() {
				factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{})

				contributor, ok, err := modules.NewContributor(factory.Build, mockPkgManager)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue())

				Expect(contributor.Contribute()).To(Succeed())

				Expect(filepath.Join(layerModulesDir, "test_module")).To(BeAnExistingFile())
				link, err := os.Readlink(filepath.Join(factory.Build.Application.Root, dir))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal(layerModulesDir))
			})
		})
	})
}
