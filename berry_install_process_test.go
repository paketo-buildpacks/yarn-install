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

func testBerryInstallProcess(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("ShouldRun", func() {
		var (
			workingDir     string
			executable     *fakes.Executable
			summer         *fakes.Summer
			buffer         *bytes.Buffer
			installProcess yarninstall.BerryInstallProcess
		)

		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			executable = &fakes.Executable{}
			summer = &fakes.Summer{}
			buffer = bytes.NewBuffer(nil)

			installProcess = yarninstall.NewBerryInstallProcess(executable, &fakes.Executable{}, summer, scribe.NewEmitter(buffer))
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
		})

		context("when yarn.lock does not exist", func() {
			it("returns run=true with empty sha", func() {
				run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{})
				Expect(err).NotTo(HaveOccurred())
				Expect(run).To(BeTrue())
				Expect(sha).To(Equal(""))
			})
		})

		context("when yarn.lock exists and sha has changed", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
				summer.SumCall.Stub = func(...string) (string, error) {
					return "new-sha", nil
				}
			})

			it("returns run=true with new sha", func() {
				run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{
					"cache_sha": "old-sha",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(run).To(BeTrue())
				Expect(sha).To(Equal("new-sha"))
				// Should sum yarn.lock, package.json and a temp env file
				Expect(summer.SumCall.Receives.Paths[0]).To(Equal(filepath.Join(workingDir, "yarn.lock")))
				Expect(summer.SumCall.Receives.Paths[1]).To(Equal(filepath.Join(workingDir, "package.json")))
			})
		})

		context("when yarn.lock exists and sha matches", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
				summer.SumCall.Stub = func(...string) (string, error) {
					return "same-sha", nil
				}
			})

			it("returns run=false", func() {
				run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{
					"cache_sha": "same-sha",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(run).To(BeFalse())
				Expect(sha).To(Equal(""))
			})
		})

		context("when cache_sha metadata is missing (first run)", func() {
			it.Before(func() {
				Expect(os.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
				summer.SumCall.Stub = func(...string) (string, error) {
					return "first-sha", nil
				}
			})

			it("returns run=true", func() {
				run, sha, err := installProcess.ShouldRun(workingDir, map[string]interface{}{})
				Expect(err).NotTo(HaveOccurred())
				Expect(run).To(BeTrue())
				Expect(sha).To(Equal("first-sha"))
			})
		})

		context("failure cases", func() {
			context("when working dir is unreadable", func() {
				it.Before(func() {
					Expect(os.Chmod(workingDir, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, _, err := installProcess.ShouldRun(workingDir, map[string]interface{}{})
					Expect(err).To(MatchError(ContainSubstring("unable to read yarn.lock file:")))
				})
			})

			context("when summer fails", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(workingDir, "yarn.lock"), []byte(""), os.ModePerm)).To(Succeed())
					summer.SumCall.Stub = func(...string) (string, error) {
						return "", errors.New("sum failed")
					}
				})

				it("returns an error", func() {
					_, _, err := installProcess.ShouldRun(workingDir, map[string]interface{}{})
					Expect(err).To(MatchError(ContainSubstring("unable to sum config files:")))
				})
			})
		})
	})

	context("SetupModules", func() {
		var (
			workingDir              string
			currentModulesLayerPath string
			nextModulesLayerPath    string
			installProcess          yarninstall.BerryInstallProcess
		)

		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			currentModulesLayerPath, err = os.MkdirTemp("", "current-modules-dir")
			Expect(err).NotTo(HaveOccurred())

			nextModulesLayerPath, err = os.MkdirTemp("", "next-modules-dir")
			Expect(err).NotTo(HaveOccurred())

			installProcess = yarninstall.NewBerryInstallProcess(&fakes.Executable{}, &fakes.Executable{}, &fakes.Summer{}, scribe.NewEmitter(bytes.NewBuffer(nil)))
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(currentModulesLayerPath)).To(Succeed())
			Expect(os.RemoveAll(nextModulesLayerPath)).To(Succeed())
		})

		context("when currentModulesLayerPath is empty (first run)", func() {
			it("creates node_modules in next layer and returns the layer path", func() {
				nextPath, err := installProcess.SetupModules(workingDir, "", nextModulesLayerPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPath).To(Equal(nextModulesLayerPath))
				Expect(filepath.Join(nextModulesLayerPath, "node_modules")).To(BeADirectory())
			})
		})

		context("when currentModulesLayerPath is set (cached layer)", func() {
			it.Before(func() {
				Expect(os.MkdirAll(filepath.Join(currentModulesLayerPath, "node_modules"), os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(currentModulesLayerPath, "node_modules", "cached-pkg"), []byte(""), os.ModePerm)).To(Succeed())
			})

			it("copies node_modules from current layer into next layer", func() {
				nextPath, err := installProcess.SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(nextPath).To(Equal(nextModulesLayerPath))
				Expect(filepath.Join(nextModulesLayerPath, "node_modules", "cached-pkg")).To(BeAnExistingFile())
			})
		})

		context("failure cases", func() {
			context("when copying from current layer fails", func() {
				it.Before(func() {
					Expect(os.Chmod(currentModulesLayerPath, 0444)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(currentModulesLayerPath, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := installProcess.SetupModules(workingDir, currentModulesLayerPath, nextModulesLayerPath)
					Expect(err).To(MatchError(ContainSubstring("failed to copy node_modules directory:")))
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
			installProcess   yarninstall.BerryInstallProcess
		)

		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			modulesLayerPath, err = os.MkdirTemp("", "modules-dir")
			Expect(err).NotTo(HaveOccurred())

			buffer = bytes.NewBuffer(nil)
			executions = []pexec.Execution{}
			executable = &fakes.Executable{}
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				executions = append(executions, execution)
				fmt.Fprintln(execution.Stdout, "stdout output")
				fmt.Fprintln(execution.Stderr, "stderr output")
				return nil
			}

			installProcess = yarninstall.NewBerryInstallProcess(executable, &fakes.Executable{}, &fakes.Summer{}, scribe.NewEmitter(buffer))
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(modulesLayerPath)).To(Succeed())
		})

		context("when no yarnPath in .yarnrc.yml (buildpack-provided Berry)", func() {
			context("when launch is false", func() {
				it("runs yarn install --immutable with YARN_IGNORE_PATH and NODE_ENV=development", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, false)
					Expect(err).NotTo(HaveOccurred())

					Expect(executions).To(HaveLen(1))
					Expect(executions[0].Args).To(Equal([]string{"install", "--immutable"}))
					Expect(executions[0].Dir).To(Equal(workingDir))
					Expect(executions[0].Env).To(ContainElement("YARN_IGNORE_PATH=1"))
					Expect(executions[0].Env).To(ContainElement("YARN_NODE_LINKER=node-modules"))
					Expect(executions[0].Env).To(ContainElement("NODE_ENV=development"))
					Expect(executions[0].Env).To(ContainElement(ContainSubstring("YARN_INSTALL_STATE_PATH=")))

					Expect(buffer.String()).To(ContainSubstring("buildpack-provided Berry"))
				})
			})

			context("when launch is true", func() {
				it("runs yarn install --immutable without overriding NODE_ENV", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, true)
					Expect(err).NotTo(HaveOccurred())

					Expect(executions).To(HaveLen(1))
					Expect(executions[0].Args).To(Equal([]string{"install", "--immutable"}))
					Expect(executions[0].Env).To(ContainElement("YARN_IGNORE_PATH=1"))
					Expect(executions[0].Env).To(ContainElement("YARN_NODE_LINKER=node-modules"))
					Expect(executions[0].Env).NotTo(ContainElement("NODE_ENV=development"))
				})
			})
		})

		context("when app provides yarnPath in .yarnrc.yml", func() {
			var nodeExecutable *fakes.Executable
			var nodeExecutions []pexec.Execution

			it.Before(func() {
				// Create a fake committed Berry binary.
				Expect(os.MkdirAll(filepath.Join(workingDir, ".yarn", "releases"), os.ModePerm)).To(Succeed())
				Expect(os.WriteFile(
					filepath.Join(workingDir, ".yarn", "releases", "yarn-4.12.0.cjs"),
					[]byte("// fake berry"), os.ModePerm,
				)).To(Succeed())
				Expect(os.WriteFile(
					filepath.Join(workingDir, ".yarnrc.yml"),
					[]byte("yarnPath: .yarn/releases/yarn-4.12.0.cjs\nnodeLinker: node-modules\n"),
					os.ModePerm,
				)).To(Succeed())

				nodeExecutions = []pexec.Execution{}
				nodeExecutable = &fakes.Executable{}
				nodeExecutable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					nodeExecutions = append(nodeExecutions, execution)
					return nil
				}
				installProcess = yarninstall.NewBerryInstallProcess(executable, nodeExecutable, &fakes.Summer{}, scribe.NewEmitter(buffer))
			})

			it("invokes node <yarnPath> install --immutable and does not set YARN_IGNORE_PATH", func() {
				err := installProcess.Execute(workingDir, modulesLayerPath, false)
				Expect(err).NotTo(HaveOccurred())

				// yarn executable must NOT be called.
				Expect(executions).To(HaveLen(0))
				// node executable IS called.
				Expect(nodeExecutions).To(HaveLen(1))
				expectedBin := filepath.Join(workingDir, ".yarn", "releases", "yarn-4.12.0.cjs")
				Expect(nodeExecutions[0].Args).To(Equal([]string{expectedBin, "install", "--immutable"}))
				Expect(nodeExecutions[0].Env).NotTo(ContainElement("YARN_IGNORE_PATH=1"))
				Expect(nodeExecutions[0].Env).To(ContainElement("YARN_NODE_LINKER=node-modules"))
				Expect(nodeExecutions[0].Env).To(ContainElement("NODE_ENV=development"))
				Expect(buffer.String()).To(ContainSubstring("app-provided yarnPath"))
			})
		})

		context("when yarn install places node_modules in the working dir", func() {
			it.Before(func() {
				executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
					executions = append(executions, execution)
					// Simulate berry creating node_modules in workingDir
					Expect(os.MkdirAll(filepath.Join(workingDir, "node_modules", "some-pkg"), os.ModePerm)).To(Succeed())
					return nil
				}
			})

			it("moves node_modules into the layer", func() {
				err := installProcess.Execute(workingDir, modulesLayerPath, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(workingDir, "node_modules")).NotTo(BeADirectory())
				Expect(filepath.Join(modulesLayerPath, "node_modules", "some-pkg")).To(BeADirectory())
			})
		})

		context("failure cases", func() {
			context("when the yarn executable fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						return errors.New("yarn berry install failed")
					}
				})

				it("returns an error", func() {
					err := installProcess.Execute(workingDir, modulesLayerPath, true)
					Expect(err).To(MatchError(ContainSubstring("failed to execute yarn install:")))
					Expect(err).To(MatchError(ContainSubstring("yarn berry install failed")))
				})
			})
		})
	})

	context("environment variable integration (NODE_ENV)", func() {
		var (
			workingDir       string
			modulesLayerPath string
			executions       []pexec.Execution
			installProcess   yarninstall.BerryInstallProcess
		)

		it.Before(func() {
			var err error
			workingDir, err = os.MkdirTemp("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			modulesLayerPath, err = os.MkdirTemp("", "modules-dir")
			Expect(err).NotTo(HaveOccurred())

			executions = []pexec.Execution{}
			exe := &fakes.Executable{}
			exe.ExecuteCall.Stub = func(e pexec.Execution) error {
				executions = append(executions, e)
				return nil
			}
			installProcess = yarninstall.NewBerryInstallProcess(exe, &fakes.Executable{}, &fakes.Summer{}, scribe.NewEmitter(bytes.NewBuffer(nil)))
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
			Expect(os.RemoveAll(modulesLayerPath)).To(Succeed())
		})

		it("does not have --frozen-lockfile, --ignore-engines, or --production", func() {
			err := installProcess.Execute(workingDir, modulesLayerPath, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Join(executions[0].Args, " ")).NotTo(ContainSubstring("frozen-lockfile"))
			Expect(strings.Join(executions[0].Args, " ")).NotTo(ContainSubstring("ignore-engines"))
			Expect(strings.Join(executions[0].Args, " ")).NotTo(ContainSubstring("production"))
		})
	})
}
