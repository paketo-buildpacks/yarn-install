package yarninstall_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testSwitchingInstallProcess(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	var (
		workingDir     string
		classicProcess *fakes.InstallProcess
		berryProcess   *fakes.InstallProcess
		switching      yarninstall.SwitchingInstallProcess
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		classicProcess = &fakes.InstallProcess{}
		berryProcess = &fakes.InstallProcess{}

		switching = yarninstall.NewSwitchingInstallProcess(classicProcess, berryProcess)
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	context("when app has no packageManager field", func() {
		it("delegates all methods to the classic process", func() {
			classicProcess.ShouldRunCall.Returns.Run = true
			classicProcess.ShouldRunCall.Returns.Sha = "classic-sha"

			run, sha, err := switching.ShouldRun(workingDir, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(run).To(BeTrue())
			Expect(sha).To(Equal("classic-sha"))
			Expect(classicProcess.ShouldRunCall.CallCount).To(Equal(1))
			Expect(berryProcess.ShouldRunCall.CallCount).To(Equal(0))

			classicProcess.SetupModulesCall.Returns.String = "/modules"
			nextPath, err := switching.SetupModules(workingDir, "", "/next")
			Expect(err).NotTo(HaveOccurred())
			Expect(nextPath).To(Equal("/modules"))
			Expect(classicProcess.SetupModulesCall.CallCount).To(Equal(1))
			Expect(berryProcess.SetupModulesCall.CallCount).To(Equal(0))

			err = switching.Execute(workingDir, "/modules", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(classicProcess.ExecuteCall.CallCount).To(Equal(1))
			Expect(berryProcess.ExecuteCall.CallCount).To(Equal(0))
		})
	})

	context("when app has packageManager yarn@1.22.22 (classic)", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "package.json"),
				[]byte(`{"packageManager":"yarn@1.22.22"}`), os.ModePerm)).To(Succeed())
		})

		it("delegates to the classic process", func() {
			_, _, err := switching.ShouldRun(workingDir, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(classicProcess.ShouldRunCall.CallCount).To(Equal(1))
			Expect(berryProcess.ShouldRunCall.CallCount).To(Equal(0))
		})
	})

	context("when app has packageManager yarn@4.14.1 (berry)", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "package.json"),
				[]byte(`{"packageManager":"yarn@4.14.1"}`), os.ModePerm)).To(Succeed())
		})

		it("delegates ShouldRun to the berry process", func() {
			berryProcess.ShouldRunCall.Returns.Run = true
			berryProcess.ShouldRunCall.Returns.Sha = "berry-sha"

			run, sha, err := switching.ShouldRun(workingDir, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(run).To(BeTrue())
			Expect(sha).To(Equal("berry-sha"))
			Expect(berryProcess.ShouldRunCall.CallCount).To(Equal(1))
			Expect(classicProcess.ShouldRunCall.CallCount).To(Equal(0))
		})

		it("delegates SetupModules to the berry process", func() {
			berryProcess.SetupModulesCall.Returns.String = "/berry-modules"
			nextPath, err := switching.SetupModules(workingDir, "", "/next")
			Expect(err).NotTo(HaveOccurred())
			Expect(nextPath).To(Equal("/berry-modules"))
			Expect(berryProcess.SetupModulesCall.CallCount).To(Equal(1))
			Expect(classicProcess.SetupModulesCall.CallCount).To(Equal(0))
		})

		it("delegates Execute to the berry process", func() {
			err := switching.Execute(workingDir, "/modules", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(berryProcess.ExecuteCall.CallCount).To(Equal(1))
			Expect(classicProcess.ExecuteCall.CallCount).To(Equal(0))
		})
	})

	context("when app has packageManager yarn@2.x (berry threshold)", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "package.json"),
				[]byte(`{"packageManager":"yarn@2.0.0"}`), os.ModePerm)).To(Succeed())
		})

		it("delegates to the berry process", func() {
			_, _, err := switching.ShouldRun(workingDir, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(berryProcess.ShouldRunCall.CallCount).To(Equal(1))
			Expect(classicProcess.ShouldRunCall.CallCount).To(Equal(0))
		})
	})

	context("error propagation", func() {
		context("when berry ShouldRun returns an error", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(workingDir, "package.json"),
					[]byte(`{"packageManager":"yarn@4.0.0"}`), os.ModePerm)).To(Succeed())
				berryProcess.ShouldRunCall.Returns.Err = errors.New("berry error")
			})

			it("returns the error", func() {
				_, _, err := switching.ShouldRun(workingDir, nil)
				Expect(err).To(MatchError("berry error"))
			})
		})

		context("when classic Execute returns an error", func() {
			it.Before(func() {
				classicProcess.ExecuteCall.Returns.Error = errors.New("classic error")
			})

			it("returns the error", func() {
				err := switching.Execute(workingDir, "/modules", true)
				Expect(err).To(MatchError("classic error"))
			})
		})
	})
}
