package yarn_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/logger"
	"github.com/cloudfoundry/yarn-cnb/yarn"

	"github.com/golang/mock/gomock"

	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

//go:generate mockgen -source=cli.go -destination=mocks_test.go -package=yarn_test

func TestUnitYarn(t *testing.T) {
	spec.Run(t, "CLI", testYarn, spec.Report(report.Terminal{}))
}

func testYarn(t *testing.T, when spec.G, it spec.S) {
	const offlineCacheDir = "npm-packages-offline-cache"

	var (
		mockCtrl   *gomock.Controller
		mockRunner *MockRunner
		log        logger.Logger
		logBuf     *bytes.Buffer
	)

	it.Before(func() {
		RegisterTestingT(t)
		mockCtrl = gomock.NewController(t)
		mockRunner = NewMockRunner(mockCtrl)
		logBuf = &bytes.Buffer{}
		log = logger.NewLogger(ioutil.Discard, logBuf)

	})

	it.After(func() {
		mockCtrl.Finish()
	})

	when("Install", func() {
		var (
			tempDir    string
			appDir     string
			modulesDir string
			cacheDir   string
			yarnBin    string
		)

		it.Before(func() {
			tempDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			modulesDir = filepath.Join(tempDir, "modules")
			appDir = filepath.Join(tempDir, "app")
			Expect(os.MkdirAll(appDir, 0777)).To(Succeed())
			cacheDir = filepath.Join(tempDir, "cache")
			yarnBin = filepath.Join("bin", "yarn")
		})

		it.After(func() {
			os.RemoveAll(tempDir)
		})

		it("yarn installs ONLINE by default", func() {
			pkgManager, err := yarn.NewCLI(appDir, yarnBin, mockRunner, log)
			Expect(err).NotTo(HaveOccurred())
			mockRunner.EXPECT().RunWithOutput(yarnBin, modulesDir, false, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", cacheDir)

			Expect(pkgManager.Install(modulesDir, cacheDir)).To(Succeed())
			Expect(logBuf.String()).To(ContainSubstring("Running yarn in online mode"))
		})

		it("yarn installs OFFLINE when the offline cache dir exists in the app", func() {
			npmCacheDir := filepath.Join(appDir, offlineCacheDir)
			Expect(os.MkdirAll(npmCacheDir, 0777)).To(Succeed())
			pkgManager, err := yarn.NewCLI(appDir, yarnBin, mockRunner, log)
			Expect(err).NotTo(HaveOccurred())
			mockRunner.EXPECT().Run(yarnBin, appDir, "config", "set", "yarn-offline-mirror", npmCacheDir)
			mockRunner.EXPECT().Run(yarnBin, appDir, "config", "set", "yarn-offline-mirror-pruning", "false")
			mockRunner.EXPECT().RunWithOutput(yarnBin, modulesDir, false, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", cacheDir, "--offline")

			Expect(pkgManager.Install(modulesDir, cacheDir)).To(Succeed())
			Expect(logBuf.String()).To(ContainSubstring("Running yarn in offline mode"))
		})

		it("warns that unmet dependencies may cause issues", func() {
			pkgManager, err := yarn.NewCLI(appDir, yarnBin, mockRunner, log)
			Expect(err).NotTo(HaveOccurred())

			mockRunner.EXPECT().RunWithOutput(yarnBin, modulesDir, false, "install", "--pure-lockfile", "--ignore-engines", "--cache-folder", cacheDir).Return("unmet peer dependency", nil)

			Expect(pkgManager.Install(modulesDir, cacheDir)).To(Succeed())

			Expect(logBuf.String()).To(ContainSubstring("Unmet dependencies don't fail yarn install but may cause runtime issues\nSee: https://github.com/npm/npm/issues/7494"))
		})
	})

	when("Check", func() {
		it("logs that yarn.lock and package.json match", func() {
			pkgManager, err := yarn.NewCLI("some-app", "some-bin", mockRunner, log)
			Expect(err).NotTo(HaveOccurred())
			mockRunner.EXPECT().RunWithOutput("some-bin", "some-app", true, "check").Return("", nil)
			Expect(pkgManager.Check("some-app")).To(Succeed())
			Expect(logBuf.String()).To(ContainSubstring("yarn.lock and package.json match"))
		})

		it("warns that yarn.lock is out of date", func() {
			pkgManager, err := yarn.NewCLI("some-app", "some-bin", mockRunner, log)
			Expect(err).NotTo(HaveOccurred())
			mockRunner.EXPECT().RunWithOutput("some-bin", "some-app", true, "check").Return("Some yarn check output", &exec.ExitError{})
			Expect(pkgManager.Check("some-app")).To(Succeed())
			Expect(logBuf.String()).To(ContainSubstring("yarn.lock is outdated"))
		})

		it("runs as offline when offline", func() {
			err := os.MkdirAll(filepath.Join("some-app", offlineCacheDir), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
			defer os.RemoveAll("some-app")

			pkgManager, err := yarn.NewCLI("some-app", "some-bin", mockRunner, log)
			Expect(err).NotTo(HaveOccurred())

			mockRunner.EXPECT().RunWithOutput("some-bin", "some-app", true, "check", "--offline")
			Expect(pkgManager.Check("some-app")).To(Succeed())
		})
	})
}
