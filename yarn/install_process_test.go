package yarn_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/packit/pexec"
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
			installProcess        yarn.YarnInstallProcess
			executions            []pexec.Execution

			executable *fakes.Executable
		)

		it.Before(func() {
			var err error
			workingDir, err = ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			layerPath, err = ioutil.TempDir("", "layer-dir")
			Expect(err).NotTo(HaveOccurred())

			executions = []pexec.Execution{}
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) (string, string, error) {
				executions = append(executions, execution)

				if strings.Contains(strings.Join(execution.Args, " "), "yarn-offline-mirror") {
					fmt.Fprintln(execution.Stdout, "undefined")
				}

				return "", "", nil
			}

			installProcess = yarn.NewYarnInstallProcess(executable)
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(layerPath)).To(Succeed())
		})

		it("executes yarn install", func() {
			err := installProcess.Execute(workingDir, layerPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(executions).To(HaveLen(2))
			Expect(executions[0].Args).To(Equal([]string{
				"config",
				"get",
				"yarn-offline-mirror",
			}))
			Expect(executions[1].Args).To(Equal([]string{
				"install",
				"--pure-lockfile",
				"--ignore-engines",
			}))

			Expect(filepath.Join(layerPath, "node_modules")).To(BeADirectory())

			link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal(filepath.Join(layerPath, "node_modules")))
		})

		context("when there is an offline mirror directory", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "offline-mirror"), os.ModePerm)).To(Succeed())

				executable.ExecuteCall.Stub = func(execution pexec.Execution) (string, string, error) {
					executions = append(executions, execution)

					if strings.Contains(strings.Join(execution.Args, " "), "yarn-offline-mirror") {
						fmt.Fprintln(execution.Stdout, filepath.Join(workingDir, "offline-mirror"))
					}

					return "", "", nil
				}
			})

			it("executes yarn install in offline mode", func() {
				err := installProcess.Execute(workingDir, layerPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(2))
				Expect(executions[0].Args).To(Equal([]string{
					"config",
					"get",
					"yarn-offline-mirror",
				}))
				Expect(executions[1].Args).To(Equal([]string{
					"install",
					"--pure-lockfile",
					"--ignore-engines",
					"--offline",
				}))

				Expect(filepath.Join(layerPath, "node_modules")).To(BeADirectory())

				link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal(filepath.Join(layerPath, "node_modules")))
			})
		})

		context("when there is a node_modules directory", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "node_modules"), os.ModePerm)).To(Succeed())

				err := ioutil.WriteFile(filepath.Join(workingDir, "node_modules", "some-file"), []byte("some-content"), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			it("executes yarn install after copying the node_modules directory to the layer", func() {
				err := installProcess.Execute(workingDir, layerPath)
				Expect(err).NotTo(HaveOccurred())

				content, err := ioutil.ReadFile(filepath.Join(layerPath, "node_modules", "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("some-content"))

				link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal(filepath.Join(layerPath, "node_modules")))
			})
		})

		context("failure cases", func() {
			context("node_modules directory cannot be created in layer directory", func() {
				it.Before(func() {
					Expect(os.Chmod(workingDir, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, layerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to create node_modules directory:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("node_modules directory cannot be created in layer directory", func() {
				it.Before(func() {
					Expect(os.Chmod(layerPath, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(layerPath, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, layerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to move node_modules directory to layer:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("the yarn executable fails to get config", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) (string, string, error) {
						if strings.Contains(strings.Join(execution.Args, " "), "config") {
							return "", "", errors.New("yarn config failed")
						}

						return "", "", nil
					}
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, layerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn config:")))
					Expect(err).To(MatchError(ContainSubstring("yarn config failed")))
				})
			})

			context("the offline mirror directory cannot be read", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) (string, string, error) {
						if strings.Contains(strings.Join(execution.Args, " "), "config") {
							return "", "", errors.New("yarn config failed")
						}

						return "", "", nil
					}
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, layerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn config:")))
					Expect(err).To(MatchError(ContainSubstring("yarn config failed")))
				})
			})

			context("the yarn executable fails to install", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) (string, string, error) {
						if strings.Contains(strings.Join(execution.Args, " "), "install") {
							return "", "", errors.New("yarn install failed")
						}

						return "", "", nil
					}
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
