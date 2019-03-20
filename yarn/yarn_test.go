package yarn_test

import (
	"bytes"
	logger2 "github.com/buildpack/libbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/logger"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	"github.com/golang/mock/gomock"

	"github.com/sclevine/spec/report"

	"github.com/cloudfoundry/libcfbuildpack/test"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

//go:generate mockgen -source=yarn.go -destination=mocks_test.go -package=yarn_test

func TestUnitYarn(t *testing.T) {
	spec.Run(t, "Yarn", testYarn, spec.Report(report.Terminal{}))
	spec.Run(t, "Yarn", testContributor, spec.Report(report.Terminal{}))
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
		RegisterTestingT(t)
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

	when("node_modules and yarn-cache already exist", func() {
		var (
			err            error
			location       string
			destination    string
			cachedModule   string
			cachedYarnItem string
			yarnBin        string
			nodeModules    string
			yarnCache      string
		)

		it.Before(func() {
			location, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			destinationRoot, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			destination = filepath.Join(destinationRoot, "node_modules")

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
			mockRunner.EXPECT().RunWithOutput(yarnBin, location, false, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", yarnCache, "--modules-folder", destination)
			mockRunner.EXPECT().Run(yarnBin, location, "check")

			Expect(pkgManager.InstallOnline(location, destination)).To(Succeed())

			Expect(filepath.Join(nodeModules, "module")).To(BeARegularFile())
			Expect(cachedModule).NotTo(BeARegularFile())

			Expect(filepath.Join(yarnCache, "cache-item")).To(BeARegularFile())
			Expect(cachedYarnItem).NotTo(BeARegularFile())
		})

		it("should install offline and reuse the existing modules + cache", func() {
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache"))
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror-pruning", "false")
			mockRunner.EXPECT().RunWithOutput(yarnBin, location, false, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", yarnCache, "--modules-folder", destination, "--offline")
			mockRunner.EXPECT().Run(yarnBin, location, "check", "--offline")

			Expect(pkgManager.InstallOffline(location, destination)).To(Succeed())

			Expect(filepath.Join(nodeModules, "module")).To(BeARegularFile())
			Expect(cachedModule).NotTo(BeARegularFile())

			Expect(filepath.Join(yarnCache, "cache-item")).To(BeARegularFile())
			Expect(cachedYarnItem).NotTo(BeARegularFile())
		})

		it("should warn if yarn.lock file is not up to date", func() {
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache"))
			mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror-pruning", "false")
			mockRunner.EXPECT().RunWithOutput(yarnBin, location, false, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", yarnCache, "--modules-folder", destination, "--offline")
			mockRunner.EXPECT().Run(yarnBin, location, "check", "--offline").Return(&exec.ExitError{})
			mockLogger.EXPECT().Warning("yarn.lock is outdated")

			Expect(pkgManager.InstallOffline(location, destination)).To(Succeed())
		})

		when("Not fully vendored", func() {
			it("warns that unmet dependencies may cause issues", func() {
				debugBuff := bytes.Buffer{}
				infoBuff := bytes.Buffer{}

				yarnLogger := logger.Logger{Logger: logger2.NewLogger(&debugBuff, &infoBuff)}
				pkgManager.Logger = yarnLogger

				mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror", filepath.Join(location, "npm-packages-offline-cache"))
				mockRunner.EXPECT().Run(yarnBin, location, "config", "set", "yarn-offline-mirror-pruning", "true")
				mockRunner.EXPECT().RunWithOutput(yarnBin, location, false, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", yarnCache, "--modules-folder", destination).Return("unmet peer dependency", nil)
				mockRunner.EXPECT().Run(yarnBin, location, "check")

				Expect(pkgManager.InstallOnline(location, destination)).To(Succeed())
				Expect(infoBuff.String()).To(ContainSubstring(yarn.UNMET_DEP_WARNING))
			})
		})
	})
}
