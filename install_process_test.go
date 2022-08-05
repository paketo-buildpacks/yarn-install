package yarninstall_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testInstallProcess(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("ShouldRun", func() {
		var (
			workingDir     string
			executable     *fakes.Executable
			installProcess yarninstall.YarnInstallProcess
			summer         *fakes.Summer
			execution      pexec.Execution
		)

		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(filepath.Join(workingDir, "config-file"), []byte("hi"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			executable = &fakes.Executable{}
			summer = &fakes.Summer{}

			executable.ExecuteCall.Stub = func(exec pexec.Execution) error {
				execution = exec
				fmt.Fprintln(exec.Stdout, "undefined")
				return nil
			}
			installProcess = yarninstall.NewYarnInstallProcess(executable, summer, scribe.NewEmitter(bytes.NewBuffer(nil)))
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

			context("when the yarn.lock or config file has changed", func() {
				it.Before(func() {
					summer.SumCall.Stub = func(...string) (string, error) {
						return "some-other-sha", nil
					}
					Expect(os.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
				})

				it("succeeds when sha is different", func() {
					run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{
						"cache_sha": "some-sha",
					})
					Expect(summer.SumCall.Receives.Paths[0]).To(Equal(filepath.Join(workingDir, "yarn.lock")))
					Expect(summer.SumCall.Receives.Paths[1]).To(ContainSubstring("config-file"))
					Expect(run).To(BeTrue())
					Expect(sha).To(Equal("some-other-sha"))
					Expect(err).NotTo(HaveOccurred())
					Expect(execution.Args).To(Equal([]string{
						"config",
						"list",
						"--silent",
					}))
					Expect(execution.Dir).To(Equal(workingDir))
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
					summer.SumCall.Stub = func(...string) (string, error) {
						return "some-sha", nil
					}
					Expect(os.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
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

				context("when yarn config list fails to execute", func() {
					it.Before(func() {
						Expect(os.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
						executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
							return errors.New("very bad error")
						}
						installProcess = yarninstall.NewYarnInstallProcess(executable, summer, scribe.NewEmitter(bytes.NewBuffer(nil)))
					})

					it("fails", func() {
						_, _, err := installProcess.ShouldRun(workingDir, map[string]interface{}{})
						Expect(err).To(MatchError(ContainSubstring("very bad error")))
						Expect(err).To(MatchError(ContainSubstring("failed to execute yarn config output")))
					})
				})
			})
		})
	})

	context("SetupModules", func() {
		var (
			workingDir              string
			tempDir                 string
			currentModulesLayerPath string
			nextModulesLayerPath    string
			buffer                  *bytes.Buffer
			executable              *fakes.Executable
			summer                  *fakes.Summer

			installProcess yarninstall.YarnInstallProcess
		)

		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			tempDir, err = os.MkdirTemp("", "temp")
			Expect(err).NotTo(HaveOccurred())

			currentModulesLayerPath, err = os.MkdirTemp("", "current-modules-dir")
			Expect(err).NotTo(HaveOccurred())

			nextModulesLayerPath, err = os.MkdirTemp("", "next-modules-dir")
			Expect(err).NotTo(HaveOccurred())

			summer = &fakes.Summer{}
			buffer = bytes.NewBuffer(nil)

			executable = &fakes.Executable{}

			installProcess = yarninstall.NewYarnInstallProcess(executable, summer, scribe.NewEmitter(buffer))
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(tempDir)).To(Succeed())
			Expect(os.RemoveAll(currentModulesLayerPath)).To(Succeed())
			Expect(os.RemoveAll(nextModulesLayerPath)).To(Succeed())
		})

		context("when the current node directory is not set", func() {
			context("when there is not a node_modules directory in the working", func() {
				it("makes a node_modules directory in the working dir and one in the next modules dir and symlinks them", func() {
					nextPath, err := installProcess.SetupModules(workingDir, "", nextModulesLayerPath, tempDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(nextPath).To(Equal(nextModulesLayerPath))

					Expect(filepath.Join(nextModulesLayerPath, "node_modules")).To(BeADirectory())

					link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
					Expect(err).NotTo(HaveOccurred())
					Expect(link).To(Equal(filepath.Join(nextModulesLayerPath, "node_modules")))
				})
			})

			context("when there is a node_modules directory in the working dir", func() {
				it.Before(func() {
					Expect(os.MkdirAll(filepath.Join(workingDir, "node_modules"), os.ModePerm)).To(Succeed())

					Expect(os.WriteFile(filepath.Join(workingDir, "node_modules", "some-file"), []byte(""), os.ModePerm)).To(Succeed())
				})
				it("moves the contents of the node_modules directory in the working dir and into the next modules dir and symlinks them", func() {
					nextPath, err := installProcess.SetupModules(workingDir, "", nextModulesLayerPath, tempDir)
					Expect(err).NotTo(HaveOccurred())
					Expect(nextPath).To(Equal(nextModulesLayerPath))

					Expect(filepath.Join(nextModulesLayerPath, "node_modules")).To(BeADirectory())
					Expect(filepath.Join(nextModulesLayerPath, "node_modules", "some-file")).To(BeAnExistingFile())

					link, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
					Expect(err).NotTo(HaveOccurred())
					Expect(link).To(Equal(filepath.Join(nextModulesLayerPath, "node_modules")))
				})
			})
		})

		context("when the current modules directory is set", func() {
			it.Before(func() {
				Expect(os.MkdirAll(filepath.Join(currentModulesLayerPath, "node_modules"), os.ModePerm)).To(Succeed())

				Expect(os.WriteFile(filepath.Join(currentModulesLayerPath, "node_modules", "some-file"), []byte(""), os.ModePerm)).To(Succeed())
			})
			it("copies the contents of the node_modules directory in the current dir into the next modules dir and creates an intermediate symlink", func() {
				nextPath, err := installProcess.SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath, tempDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPath).To(Equal(nextModulesLayerPath))

				Expect(filepath.Join(currentModulesLayerPath, "node_modules")).To(BeADirectory())
				Expect(filepath.Join(currentModulesLayerPath, "node_modules", "some-file")).To(BeAnExistingFile())

				Expect(filepath.Join(nextModulesLayerPath, "node_modules")).To(BeADirectory())
				Expect(filepath.Join(nextModulesLayerPath, "node_modules", "some-file")).To(BeAnExistingFile())

				link, err := os.Readlink(filepath.Join(tempDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal(filepath.Join(currentModulesLayerPath, "node_modules")))

				link, err = os.Readlink(filepath.Join(workingDir, "node_modules"))
				Expect(err).NotTo(HaveOccurred())
				Expect(link).To(Equal(filepath.Join(tempDir, "node_modules")))
			})
		})

		context("failure cases", func() {
			context("when the node_module copy fails", func() {
				it.Before(func() {
					Expect(os.Chmod(currentModulesLayerPath, 0444)).To(Succeed())
				})
				it.After(func() {
					Expect(os.Chmod(currentModulesLayerPath, os.ModePerm)).To(Succeed())
				})
				it("returns an error", func() {
					_, err := installProcess.SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath, tempDir)
					Expect(err).To(MatchError(ContainSubstring("failed to copy node_modules directory")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("node_modules dir cannot be removed from working directory", func() {
				it.Before(func() {
					Expect(os.Chmod(workingDir, 0000)).To(Succeed())
					Expect(os.MkdirAll(filepath.Join(currentModulesLayerPath, "node_modules"), os.ModePerm)).To(Succeed())

					Expect(os.WriteFile(filepath.Join(currentModulesLayerPath, "node_modules", "some-file"), []byte(""), os.ModePerm)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := installProcess.SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath, tempDir)
					Expect(err).To(MatchError(ContainSubstring("failed to remove node_modules directory:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("node_modules directory cannot be created in working directory", func() {
				it.Before(func() {
					Expect(os.Chmod(workingDir, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := installProcess.SetupModules(workingDir, "", nextModulesLayerPath, tempDir)
					Expect(err).To(MatchError(ContainSubstring("failed to create node_modules directory:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})

			context("node_modules directory cannot be created in layer directory", func() {
				it.Before(func() {
					Expect(os.Chmod(nextModulesLayerPath, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(nextModulesLayerPath, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := installProcess.SetupModules(workingDir, "", nextModulesLayerPath, tempDir)
					Expect(err).To(MatchError(ContainSubstring("failed to move node_modules directory to layer:")))
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})
		})
	})

	context("Execute", func() {
		var (
			workingDir       string
			modulesLayerPath string
			executions       []pexec.Execution
			buffer           *bytes.Buffer
			executable       *fakes.Executable
			summer           *fakes.Summer

			installProcess yarninstall.YarnInstallProcess
		)

		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			modulesLayerPath, err = os.MkdirTemp("", "modules-dir")
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

			installProcess = yarninstall.NewYarnInstallProcess(executable, summer, scribe.NewEmitter(buffer))
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(modulesLayerPath)).To(Succeed())
		})

		context("when launch is false", func() {
			it("executes yarn install", func() {
				err := installProcess.Execute(workingDir, modulesLayerPath, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(2))
				Expect(executions[0].Args).To(Equal([]string{
					"config",
					"get",
					"yarn-offline-mirror",
				}))
				Expect(executions[0].Env).To(ContainElement(MatchRegexp(`^PATH=.*:node_modules/.bin$`)))
				Expect(executions[0].Dir).To(Equal(workingDir))

				Expect(executions[1].Args).To(Equal([]string{
					"install",
					"--ignore-engines",
					"--frozen-lockfile",
					"--production", "false",
					"--modules-folder",
					filepath.Join(modulesLayerPath, "node_modules"),
				}))
				Expect(executions[1].Env).To(ContainElement(MatchRegexp(`^PATH=.*:node_modules/.bin$`)))
				Expect(executions[1].Dir).To(Equal(workingDir))
			})
		})

		context("when launch is true", func() {
			it("executes yarn install", func() {
				err := installProcess.Execute(workingDir, modulesLayerPath, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(2))
				Expect(executions[0].Args).To(Equal([]string{
					"config",
					"get",
					"yarn-offline-mirror",
				}))
				Expect(executions[0].Env).To(ContainElement(MatchRegexp(`^PATH=.*:node_modules/.bin$`)))
				Expect(executions[0].Dir).To(Equal(workingDir))

				Expect(executions[1].Args).To(Equal([]string{
					"install",
					"--ignore-engines",
					"--frozen-lockfile",
					"--modules-folder",
					filepath.Join(modulesLayerPath, "node_modules"),
				}))
				Expect(executions[1].Env).To(ContainElement(MatchRegexp(`^PATH=.*:node_modules/.bin$`)))
				Expect(executions[1].Dir).To(Equal(workingDir))

			})
		})

		context("when there is an offline mirror directory", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "offline-mirror"), os.ModePerm)).To(Succeed())

				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)

					if strings.Contains(strings.Join(execution.Args, " "), "yarn-offline-mirror") {
						fmt.Fprintln(execution.Stdout, "warning some extraneous warning")
						fmt.Fprintln(execution.Stdout, "warning some other warning")
						fmt.Fprintln(execution.Stdout, filepath.Join(workingDir, "offline-mirror"))
					}

					return nil
				}
			})

			it("executes yarn install in offline mode", func() {
				err := installProcess.Execute(workingDir, modulesLayerPath, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(executions).To(HaveLen(2))
				Expect(executions[0].Args).To(Equal([]string{
					"config",
					"get",
					"yarn-offline-mirror",
				}))
				Expect(executions[0].Env).To(ContainElement(MatchRegexp(`^PATH=.*:node_modules/.bin$`)))
				Expect(executions[0].Dir).To(Equal(workingDir))

				Expect(executions[1].Args).To(Equal([]string{
					"install",
					"--ignore-engines",
					"--frozen-lockfile",
					"--offline",
					"--modules-folder",
					filepath.Join(modulesLayerPath, "node_modules"),
				}))
				Expect(executions[1].Env).To(ContainElement(MatchRegexp(`^PATH=.*:node_modules/.bin$`)))
				Expect(executions[1].Dir).To(Equal(workingDir))
			})
		})

		context("failure cases", func() {
			context("the yarn executable fails to get config", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "config") {
							fmt.Fprintf(execution.Stdout, "some stdout error")
							fmt.Fprintf(execution.Stderr, "some stderr error")
							return errors.New("yarn config failed")
						}

						return nil
					}
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, true)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn config")))
					Expect(err).To(MatchError(ContainSubstring("some stdout error")))
					Expect(err).To(MatchError(ContainSubstring("some stderr error")))
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
					err := installProcess.Execute(workingDir, modulesLayerPath, true)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn config")))
					Expect(err).To(MatchError(ContainSubstring("yarn config failed")))
				})
			})

			context("the yarn executable fails to install", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						if strings.Contains(strings.Join(execution.Args, " "), "install") {
							_, err := execution.Stdout.Write([]byte("stdout output"))
							Expect(err).NotTo(HaveOccurred())
							_, err = execution.Stderr.Write([]byte("stderr output"))
							Expect(err).NotTo(HaveOccurred())

							return errors.New("yarn install failed")
						}

						return nil
					}
				})

				it("prints the execution output and returns an error", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, true)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn install:")))
					Expect(err).To(MatchError(ContainSubstring("yarn install failed")))

					Expect(buffer.String()).To(ContainSubstring("stdout output"))
					Expect(buffer.String()).To(ContainSubstring("stderr output"))
				})
			})
		})
	})
}
