package modules_test

import (
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/cloudfoundry/yarn-cnb/modules"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

//go:generate mockgen -source=modules.go -destination=mocks_test.go -package=modules_test

func TestUnitModules(t *testing.T) {
	RegisterTestingT(t)
	spec.Run(t, "Modules", testModules, spec.Report(report.Terminal{}))
}

func testModules(t *testing.T, when spec.G, it spec.S) {
	when("modules.NewContributor", func() {
		var (
			mockCtrl       *gomock.Controller
			mockPkgManager *MockPackageManager
			factory        *test.BuildFactory
		)

		it.Before(func() {
			mockCtrl = gomock.NewController(t)
			mockPkgManager = NewMockPackageManager(mockCtrl)

			factory = test.NewBuildFactory(t)
		})

		it.After(func() {
			mockCtrl.Finish()
		})

		when("there is no yarn.lock", func() {
			it("fails", func() {
				factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{})

				_, _, err := modules.NewContributor(factory.Build, mockPkgManager)
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

				_, willContribute, err := modules.NewContributor(factory.Build, mockPkgManager)
				Expect(err).NotTo(HaveOccurred())
				Expect(willContribute).To(BeTrue())
			})

			it("returns false if a build plan does not exist with the dep", func() {
				_, willContribute, err := modules.NewContributor(factory.Build, mockPkgManager)
				Expect(err).NotTo(HaveOccurred())
				Expect(willContribute).To(BeFalse())
			})

			it("uses yarn.lock for identity", func() {
				factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{})

				contributor, _, _ := modules.NewContributor(factory.Build, mockPkgManager)
				name, version := contributor.Metadata.Identity()
				Expect(name).To(Equal(modules.Dependency))
				Expect(version).To(Equal("6a896d7017d636a532a914536a1cb7212c5d95a6ec5826d01e2b292e3a5d0a2a"))
			})

			when("the app is vendored", func() {
				it.Before(func() {
					file := filepath.Join(factory.Build.Application.Root, "npm-packages-offline-cache", "test_module")
					Expect(helper.WriteFile(file, 0666, "some module")).To(Succeed())

					mockPkgManager.EXPECT().InstallOffline(factory.Build.Application.Root).Do(func(location string) {
						module := filepath.Join(location, yarn.ModulesDir, "test_module")
						Expect(helper.WriteFile(module, 0666, "some module")).To(Succeed())

						cacheItem := filepath.Join(location, yarn.CacheDir, "cache-item")
						Expect(helper.WriteFile(cacheItem, 0666, "some module")).To(Succeed())
					})
				})

				it("contributes modules for the build phase", func() {
					factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{
						Metadata: buildplan.Metadata{"build": true},
					})

					contributor, _, err := modules.NewContributor(factory.Build, mockPkgManager)
					Expect(err).NotTo(HaveOccurred())

					Expect(contributor.Contribute()).To(Succeed())

					layer := factory.Build.Layers.Layer(modules.Dependency)
					Expect(layer).To(test.HaveLayerMetadata(true, true, false))

					Expect(filepath.Join(layer.Root, yarn.ModulesDir, "test_module")).To(BeARegularFile())
					Expect(filepath.Join(layer.Root, yarn.CacheDir, "cache-item")).To(BeARegularFile())

					Expect(layer).To(test.HaveOverrideSharedEnvironment("NODE_PATH", layer.Root))
					Expect(layer).To(test.HaveOverrideSharedEnvironment("npm_config_nodedir", ""))

					Expect(filepath.Join(factory.Build.Application.Root, yarn.ModulesDir)).NotTo(BeADirectory())
					Expect(filepath.Join(factory.Build.Application.Root, yarn.CacheDir)).NotTo(BeADirectory())
				})

				it("contributes modules for the launch phase", func() {
					factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{
						Metadata: buildplan.Metadata{"launch": true},
					})

					contributor, _, err := modules.NewContributor(factory.Build, mockPkgManager)
					Expect(err).NotTo(HaveOccurred())

					Expect(contributor.Contribute()).To(Succeed())

					layer := factory.Build.Layers.Layer(modules.Dependency)
					Expect(layer).To(test.HaveLayerMetadata(false, true, true))

					Expect(filepath.Join(layer.Root, yarn.ModulesDir, "test_module")).To(BeARegularFile())
					Expect(filepath.Join(layer.Root, yarn.CacheDir, "cache-item")).To(BeARegularFile())

					Expect(layer).To(test.HaveOverrideSharedEnvironment("NODE_PATH", layer.Root))
					Expect(layer).To(test.HaveOverrideSharedEnvironment("npm_config_nodedir", ""))

					Expect(filepath.Join(factory.Build.Application.Root, yarn.ModulesDir)).NotTo(BeADirectory())
					Expect(filepath.Join(factory.Build.Application.Root, yarn.CacheDir)).NotTo(BeADirectory())
				})
			})

			when("the app is not vendored", func() {
				it.Before(func() {
					mockPkgManager.EXPECT().InstallOnline(factory.Build.Application.Root).Do(func(location string) {
						module := filepath.Join(location, yarn.ModulesDir, "test_module")
						Expect(helper.WriteFile(module, 0666, "some module")).To(Succeed())

						cacheItem := filepath.Join(location, yarn.CacheDir, "cache-item")
						Expect(helper.WriteFile(cacheItem, 0666, "some module")).To(Succeed())
					})
				})

				it("contributes modules for the build phase", func() {
					factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{
						Metadata: buildplan.Metadata{"build": true},
					})

					contributor, _, err := modules.NewContributor(factory.Build, mockPkgManager)
					Expect(err).NotTo(HaveOccurred())

					Expect(contributor.Contribute()).To(Succeed())

					layer := factory.Build.Layers.Layer(modules.Dependency)
					Expect(layer).To(test.HaveLayerMetadata(true, true, false))

					Expect(filepath.Join(layer.Root, yarn.ModulesDir, "test_module")).To(BeARegularFile())
					Expect(filepath.Join(layer.Root, yarn.CacheDir, "cache-item")).To(BeARegularFile())

					Expect(layer).To(test.HaveOverrideSharedEnvironment("NODE_PATH", layer.Root))
					Expect(layer).To(test.HaveOverrideSharedEnvironment("npm_config_nodedir", ""))

					Expect(filepath.Join(factory.Build.Application.Root, yarn.ModulesDir)).NotTo(BeADirectory())
					Expect(filepath.Join(factory.Build.Application.Root, yarn.CacheDir)).NotTo(BeADirectory())
				})

				it("contributes modules for the launch phase", func() {
					factory.AddBuildPlan(modules.Dependency, buildplan.Dependency{
						Metadata: buildplan.Metadata{"launch": true},
					})

					contributor, _, err := modules.NewContributor(factory.Build, mockPkgManager)
					Expect(err).NotTo(HaveOccurred())

					Expect(contributor.Contribute()).To(Succeed())

					layer := factory.Build.Layers.Layer(modules.Dependency)
					Expect(layer).To(test.HaveLayerMetadata(false, true, true))

					Expect(filepath.Join(layer.Root, yarn.ModulesDir, "test_module")).To(BeARegularFile())
					Expect(filepath.Join(layer.Root, yarn.CacheDir, "cache-item")).To(BeARegularFile())

					Expect(layer).To(test.HaveOverrideSharedEnvironment("NODE_PATH", layer.Root))
					Expect(layer).To(test.HaveOverrideSharedEnvironment("npm_config_nodedir", ""))

					Expect(filepath.Join(factory.Build.Application.Root, yarn.ModulesDir)).NotTo(BeADirectory())
					Expect(filepath.Join(factory.Build.Application.Root, yarn.CacheDir)).NotTo(BeADirectory())
				})
			})
		})
	})
}
