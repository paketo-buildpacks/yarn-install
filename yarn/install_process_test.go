package yarn_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/yarn-cnb/yarn"
	"github.com/cloudfoundry/yarn-cnb/yarn/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testInstallProcess(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Execute", func() {
		var (
			workingDir, layerPath string

			installProcess yarn.YarnInstallProcess

			executable *fakes.Executable
		)

		it.Before(func() {
			var err error
			executable = &fakes.Executable{}

			workingDir, err = ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			layerPath, err = ioutil.TempDir("", "layer-dir")
			Expect(err).NotTo(HaveOccurred())

			installProcess = yarn.NewYarnInstallProcess(executable)
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(layerPath)).To(Succeed())
		})

		it("executes yarn install", func() {
			var err error

			err = installProcess.Execute(workingDir, layerPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(executable.ExecuteCall.Receives.Execution.Args).To(Equal([]string{
				"install",
				"--pure-lockfile",
				"--ignore-engines",
			}))

			Expect(filepath.Join(layerPath, "node_modules")).To(BeADirectory())

			link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal(filepath.Join(layerPath, "node_modules")))
		})

		context("failure cases", func() {
			context("node_modules directory cannot be created in layer directory", func() {
				it.Before(func() {
					Expect(os.Chmod(layerPath, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(layerPath, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, layerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to create node_modules directory:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("node_modules within layer directory cannot be symlinked into working directory", func() {
				it.Before(func() {
					Expect(os.Chmod(workingDir, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, layerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to symlink node_modules into working directory:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("the yarn executable fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Returns.Err = errors.New("yarn install failed")
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, layerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn install:")))
					Expect(err).To(MatchError(ContainSubstring("yarn install failed")))
				})
			})
		})
	})
}
