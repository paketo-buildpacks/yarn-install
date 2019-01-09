package yarn_test

import (
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	"github.com/golang/mock/gomock"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/sclevine/spec/report"

	"github.com/cloudfoundry/libcfbuildpack/test"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

//go:generate mockgen -source=yarn.go -destination=mocks_test.go -package=yarn_test

func TestUnitYarn(t *testing.T) {
	RegisterTestingT(t)
	spec.Run(t, "Yarn", testYarn, spec.Report(report.Terminal{}))
}

func testYarn(t *testing.T, when spec.G, it spec.S) {
	var (
		mockCtrl   *gomock.Controller
		mockRunner *MockRunner
		mockLogger *MockLogger
		layer      layers.Layer
		pkgManager yarn.Yarn
	)

	it.Before(func() {
		mockCtrl = gomock.NewController(t)
		mockRunner = NewMockRunner(mockCtrl)
		mockLogger = NewMockLogger(mockCtrl)

		mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

		f := test.NewBuildFactory(t)
		layer = f.Build.Layers.Layer(yarn.Dependency)
		pkgManager = yarn.Yarn{Runner: mockRunner, Logger: mockLogger, Layer: layer}
	})

	it.After(func() {
		mockCtrl.Finish()
	})

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

	when("node_modules and yarn-cache already exist", func() {
		var (
			err            error
			location       string
			cachedModule   string
			cachedYarnItem string
			yarnBin        string
			nodeModules    string
			yarnCache      string
		)

		it.Before(func() {
			location, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			yarnBin = filepath.Join(layer.Root, "bin", "yarn")
			nodeModules = filepath.Join(location, yarn.ModulesDir)
			yarnCache = filepath.Join(location, yarn.CacheDir)

			cachedModule = filepath.Join(layer.Root, yarn.ModulesDir, "module")
			Expect(os.MkdirAll(filepath.Join(layer.Root, yarn.ModulesDir), os.ModePerm)).To(Succeed())
			Expect(ioutil.WriteFile(cachedModule, []byte(""), os.ModePerm)).To(Succeed())

			cachedYarnItem = filepath.Join(layer.Root, yarn.CacheDir, "cache-item")
			Expect(os.MkdirAll(filepath.Join(layer.Root, yarn.CacheDir), os.ModePerm)).To(Succeed())
			Expect(ioutil.WriteFile(cachedYarnItem, []byte(""), os.ModePerm)).To(Succeed())
		})

		it.After(func() {
			os.RemoveAll(location)
		})

		it("should install online and reuse the existing modules + cache", func() {
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache"))
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror-pruning", "true")
			mockRunner.EXPECT().Run(yarnBin, location, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", yarnCache, "--modules-folder", nodeModules)
			mockRunner.EXPECT().Run(yarnBin, location, "check")

			Expect(pkgManager.InstallOnline(location)).To(Succeed())

			Expect(filepath.Join(nodeModules, "module")).To(BeARegularFile())
			Expect(cachedModule).NotTo(BeARegularFile())

			Expect(filepath.Join(yarnCache, "cache-item")).To(BeARegularFile())
			Expect(cachedYarnItem).NotTo(BeARegularFile())
		})

		it("should install offline and reuse the existing modules + cache", func() {
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache"))
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror-pruning", "false")
			mockRunner.EXPECT().Run(yarnBin, location, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", yarnCache, "--modules-folder", nodeModules, "--offline")
			mockRunner.EXPECT().Run(yarnBin, location, "check", "--offline")

			Expect(pkgManager.InstallOffline(location)).To(Succeed())

			Expect(filepath.Join(nodeModules, "module")).To(BeARegularFile())
			Expect(cachedModule).NotTo(BeARegularFile())

			Expect(filepath.Join(yarnCache, "cache-item")).To(BeARegularFile())
			Expect(cachedYarnItem).NotTo(BeARegularFile())
		})
	})
}
