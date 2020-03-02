package yarn_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/packit/pexec"
	"github.com/cloudfoundry/packit/scribe"
	"github.com/cloudfoundry/yarn-cnb/yarn"
	"github.com/cloudfoundry/yarn-cnb/yarn/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testInstallProcess(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("ShouldRun", func() {
		var (
			workingDir     string
			executable     *fakes.Executable
			installProcess yarn.YarnInstallProcess
			summer         *fakes.Summer
		)

		it.Before(func() {
			var err error
			workingDir, err = ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			executable = &fakes.Executable{}
			summer = &fakes.Summer{}

			installProcess = yarn.NewYarnInstallProcess(executable, summer, scribe.NewLogger(bytes.NewBuffer(nil)))
		})

		context("we should run yarn install when", func() {
			context("there is no yarn.lock file in the workingDir", func() {
				it("succeeds", func() {
					run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{
						"cache_sha": "some-sha",
					})

					Expect(run).To(BeTrue())
					Expect(sha).To(Equal(""))
					Expect(err).NotTo(HaveOccurred())
				})
			})

			context("when the yarn.lock file has a different sha", func() {
				it.Before(func() {
					summer.SumCall.Stub = func(string) (string, error) {
						return "some-other-sha", nil
					}
					Expect(ioutil.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
				})

				it("succeeds when sha is different", func() {
					run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{
						"cache_sha": "some-sha",
					})
					Expect(run).To(BeTrue())
					Expect(sha).To(Equal("some-other-sha"))
					Expect(err).NotTo(HaveOccurred())
				})

				it("succeeds when sha is missing", func() {
					run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{})
					Expect(run).To(BeTrue())
					Expect(sha).To(Equal("some-other-sha"))
					Expect(err).NotTo(HaveOccurred())
				})
			})

			context("when the sha of yarn.lock and metadata sha match", func() {
				it.Before(func() {
					summer.SumCall.Stub = func(string) (string, error) {
						return "some-sha", nil
					}
					Expect(ioutil.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
				})

				it("does not run install", func() {
					run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{
						"cache_sha": "some-sha",
					})
					Expect(run).To(BeFalse())
					Expect(sha).To(Equal(""))
					Expect(err).NotTo(HaveOccurred())
				})
			})

			context("failure cases", func() {
				context("when working dir is un-readable", func() {
					it.Before(func() {
						Expect(os.Chmod(workingDir, 0000)).To(Succeed())
					})

					it.After(func() {
						Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
					})

					it("fails", func() {
						_, _, err := installProcess.ShouldRun(workingDir, map[string]interface{}{})
						Expect(err).To(MatchError(ContainSubstring("unable to read yarn.lock file:")))
					})
				})
			})
		})
	})

	context("Execute", func() {
		var (
			workingDir       string
			modulesLayerPath string
			yarnLayerPath    string
			path             string
			executions       []pexec.Execution
			buffer           *bytes.Buffer
			executable       *fakes.Executable
			summer           *fakes.Summer

			installProcess yarn.YarnInstallProcess
		)

		it.Before(func() {
			var err error
			workingDir, err = ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			modulesLayerPath, err = ioutil.TempDir("", "modules-dir")
			Expect(err).NotTo(HaveOccurred())

			yarnLayerPath, err = ioutil.TempDir("", "yarn-dir")
			Expect(err).NotTo(HaveOccurred())

			summer = &fakes.Summer{}
			buffer = bytes.NewBuffer(nil)

			executions = []pexec.Execution{}
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				executions = append(executions, execution)

				if strings.Contains(strings.Join(execution.Args, " "), "yarn-offline-mirror") {
					fmt.Fprintln(execution.Stdout, "undefined")
				}

				return nil
			}

			path = os.Getenv("PATH")
			os.Setenv("PATH", "/some/bin")

			installProcess = yarn.NewYarnInstallProcess(executable, summer, scribe.NewLogger(buffer))
		})

		it.After(func() {
			os.Setenv("PATH", path)

			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(modulesLayerPath)).To(Succeed())
			Expect(os.RemoveAll(yarnLayerPath)).To(Succeed())
		})

		it("executes yarn install", func() {
			err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
			Expect(err).NotTo(HaveOccurred())

			Expect(executions).To(HaveLen(2))
			Expect(executions[0].Args).To(Equal([]string{
				"config",
				"get",
				"yarn-offline-mirror",
			}))
			Expect(executions[0].Env).To(ContainElement(MatchRegexp(fmt.Sprintf(`^PATH=.*:%s/bin:node_modules/.bin$`, yarnLayerPath))))

			Expect(executions[1].Args).To(Equal([]string{
				"install",
				"--ignore-engines",
				"--frozen-lockfile",
				"--modules-folder",
				filepath.Join(modulesLayerPath, "node_modules"),
			}))
			Expect(executions[1].Env).To(ContainElement(MatchRegexp(`^PATH=.*:node_modules/.bin$`)))

			Expect(filepath.Join(modulesLayerPath, "node_modules")).To(BeADirectory())

			link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal(filepath.Join(modulesLayerPath, "node_modules")))

			Expect(os.Getenv("PATH")).To(Equal(fmt.Sprintf("/some/bin:%s/bin", yarnLayerPath)))
		})

		context("when there is an offline mirror directory", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "offline-mirror"), os.ModePerm)).To(Succeed())

				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)

					if strings.Contains(strings.Join(execution.Args, " "), "yarn-offline-mirror") {
						fmt.Fprintln(execution.Stdout, filepath.Join(workingDir, "offline-mirror"))
					}

					return nil
				}
			})

			it("executes yarn install in offline mode", func() {
				err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(2))
				Expect(executions[0].Args).To(Equal([]string{
					"config",
					"get",
					"yarn-offline-mirror",
				}))
				Expect(executions[0].Env).To(ContainElement(MatchRegexp(fmt.Sprintf(`^PATH=.*:%s/bin:node_modules/.bin$`, yarnLayerPath))))

				Expect(executions[1].Args).To(Equal([]string{
					"install",
					"--ignore-engines",
					"--frozen-lockfile",
					"--offline",
					"--modules-folder",
					filepath.Join(modulesLayerPath, "node_modules"),
				}))
				Expect(executions[1].Env).To(ContainElement(MatchRegexp(fmt.Sprintf(`^PATH=.*:%s/bin:node_modules/.bin$`, yarnLayerPath))))

				Expect(filepath.Join(modulesLayerPath, "node_modules")).To(BeADirectory())

				link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal(filepath.Join(modulesLayerPath, "node_modules")))
			})
		})

		context("when there is a node_modules directory", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "node_modules"), os.ModePerm)).To(Succeed())

				err := ioutil.WriteFile(filepath.Join(workingDir, "node_modules", "some-file"), []byte("some-content"), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			it("executes yarn install after copying the node_modules directory to the layer", func() {
				err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
				Expect(err).NotTo(HaveOccurred())

				content, err := ioutil.ReadFile(filepath.Join(modulesLayerPath, "node_modules", "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(content)).To(Equal("some-content"))

				link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal(filepath.Join(modulesLayerPath, "node_modules")))
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
					err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to create node_modules directory:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("node_modules directory cannot be created in layer directory", func() {
				it.Before(func() {
					Expect(os.Chmod(modulesLayerPath, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(modulesLayerPath, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to move node_modules directory to layer:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("the yarn executable fails to get config", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "config") {
							return errors.New("yarn config failed")
						}

						return nil
					}
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn config:")))
					Expect(err).To(MatchError(ContainSubstring("yarn config failed")))
				})
			})

			context("the offline mirror directory cannot be read", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "config") {
							return errors.New("yarn config failed")
						}

						return nil
					}
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn config:")))
					Expect(err).To(MatchError(ContainSubstring("yarn config failed")))
				})
			})

			context("the yarn executable fails to install", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "install") {
							execution.Stdout.Write([]byte("stdout output"))
							execution.Stderr.Write([]byte("stderr output"))

							return errors.New("yarn install failed")
						}

						return nil
					}
				})

				it("prints the execution output and returns an error", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, yarnLayerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn install:")))
					Expect(err).To(MatchError(ContainSubstring("yarn install failed")))

					Expect(buffer.String()).To(ContainSubstring("stdout output"))
					Expect(buffer.String()).To(ContainSubstring("stderr output"))
				})
			})
		})
	})
}
