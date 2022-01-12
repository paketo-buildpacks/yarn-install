package yarninstall_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/scribe"
	"github.com/paketo-buildpacks/packit/servicebindings"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	type resolveCallParams struct {
		Typ         string
		Provider    string
		PlatformDir string
	}

	type linkCallParams struct {
		Oldname string
		Newname string
	}

	var (
		Expect = NewWithT(t).Expect

		layersDir  string
		workingDir string
		cnbDir     string
		timestamp  string

		buffer              *bytes.Buffer
		clock               chronos.Clock
		installProcess      *fakes.InstallProcess
		now                 time.Time
		pathParser          *fakes.PathParser
		bindingResolver     *fakes.BindingResolver
		bindingResolveCalls []resolveCallParams
		symlinker           *fakes.SymlinkManager
		linkCalls           []linkCallParams
		unlinkPaths         []string

		build packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = ioutil.TempDir("", "layers")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = ioutil.TempDir("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())

		cnbDir, err = ioutil.TempDir("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		now = time.Now()
		clock = chronos.NewClock(func() time.Time {
			return now
		})

		timestamp = now.Format(time.RFC3339Nano)

		installProcess = &fakes.InstallProcess{}
		installProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
			return true, "some-awesome-shasum", nil
		}

		buffer = bytes.NewBuffer(nil)

		pathParser = &fakes.PathParser{}
		pathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "some-project-dir")

		bindingResolver = &fakes.BindingResolver{}

		bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
			bindingResolveCalls = append(bindingResolveCalls, resolveCallParams{
				Typ:         typ,
				Provider:    provider,
				PlatformDir: platform,
			})
			return nil, nil
		}
		symlinker = &fakes.SymlinkManager{}
		symlinker.LinkCall.Stub = func(o, n string) error {
			linkCalls = append(linkCalls, linkCallParams{
				Oldname: o,
				Newname: n,
			})
			return nil
		}
		symlinker.UnlinkCall.Stub = func(p string) error {
			unlinkPaths = append(unlinkPaths, p)
			return nil
		}

		build = yarninstall.Build(
			pathParser,
			bindingResolver,
			symlinker,
			installProcess,
			clock,
			scribe.NewLogger(buffer),
		)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	context("when adding modules layer to image", func() {
		it("resolves and calls the build process", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Layers:     packit.Layers{Path: layersDir},
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node_modules",
							Metadata: map[string]interface{}{
								"build": true,
							},
						},
					},
				},
				Stack: "some-stack",
				Platform: packit.Platform{
					Path: "some-platform-path",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
				Layers: []packit.Layer{
					{
						Name: "modules",
						Path: filepath.Join(layersDir, "modules"),
						SharedEnv: packit.Environment{
							"PATH.append": filepath.Join(layersDir, "modules", "node_modules", ".bin"),
							"PATH.delim":  ":",
						},
						BuildEnv:         packit.Environment{},
						LaunchEnv:        packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            true,
						Cache:            true,
						Metadata: map[string]interface{}{
							"built_at":  timestamp,
							"cache_sha": "some-awesome-shasum",
						},
					},
				},
			}))

			Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))
			Expect(bindingResolver.ResolveCall.CallCount).To(Equal(2))

			Expect(bindingResolveCalls[0].Typ).To(Equal("npmrc"))
			Expect(bindingResolveCalls[0].Provider).To(Equal(""))
			Expect(bindingResolveCalls[0].PlatformDir).To(Equal("some-platform-path"))
			Expect(bindingResolveCalls[1].Typ).To(Equal("yarnrc"))
			Expect(bindingResolveCalls[1].Provider).To(Equal(""))
			Expect(bindingResolveCalls[1].PlatformDir).To(Equal("some-platform-path"))

			Expect(symlinker.LinkCall.CallCount).To(BeZero())
			Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "modules")))
		})
	})

	context("when re-using previous modules layer", func() {
		it.Before(func() {
			installProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
				return false, "", nil
			}
		})

		it("does not redo the build process", func() {
			result, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Layers:     packit.Layers{Path: layersDir},
				Stack:      "some-stack",
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node_modules",
							Metadata: map[string]interface{}{
								"build": true,
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
				Layers: []packit.Layer{
					{
						Name:             "modules",
						Path:             filepath.Join(layersDir, "modules"),
						SharedEnv:        packit.Environment{},
						BuildEnv:         packit.Environment{},
						LaunchEnv:        packit.Environment{},
						ProcessLaunchEnv: map[string]packit.Environment{},
						Build:            true,
						Cache:            true,
					},
				},
			}))
			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
			Expect(symlinker.LinkCall.Receives.Oldname).To(Equal(filepath.Join(layersDir, "modules", "node_modules")))
			Expect(symlinker.LinkCall.Receives.Newname).To(Equal(filepath.Join(workingDir, "some-project-dir", "node_modules")))
		})
	})

	context("when an npmrc service binding is provided", func() {
		it.Before(func() {
			bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
				if typ == "npmrc" {
					return []servicebindings.Binding{
						servicebindings.Binding{
							Name: "first",
							Type: "npmrc",
							Path: "some-binding-path",
							Entries: map[string]*servicebindings.Entry{
								".npmrc": servicebindings.NewEntry("some-path"),
							},
						},
					}, nil
				}
				return nil, nil
			}
		})

		it("symlinks the provided .npmrc to $HOME/.npmrc in the build container", func() {
			_, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Layers:     packit.Layers{Path: layersDir},
				Stack:      "some-stack",
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node_modules",
							Metadata: map[string]interface{}{
								"build": true,
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
			Expect(linkCalls[0].Oldname).To(Equal(filepath.Join("some-binding-path", ".npmrc")))
			home, err := os.UserHomeDir()
			Expect(err).NotTo(HaveOccurred())
			Expect(linkCalls[0].Newname).To(Equal(filepath.Join(home, ".npmrc")))
			Expect(symlinker.UnlinkCall.CallCount).To(Equal(2))
			Expect(unlinkPaths[0]).To(Equal(filepath.Join(home, ".npmrc")))
		})
	})

	context("when an yarnrc service binding is provided", func() {
		it.Before(func() {
			bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
				if typ == "yarnrc" {
					return []servicebindings.Binding{
						servicebindings.Binding{
							Name: "first",
							Type: "yarnrc",
							Path: "some-binding-path",
							Entries: map[string]*servicebindings.Entry{
								".yarnrc": servicebindings.NewEntry("some-path"),
							},
						},
					}, nil
				}
				return nil, nil
			}
		})

		it("symlinks the provided .yarnrc to $HOME/.yarnrc in the build container", func() {
			_, err := build(packit.BuildContext{
				WorkingDir: workingDir,
				CNBPath:    cnbDir,
				Layers:     packit.Layers{Path: layersDir},
				Stack:      "some-stack",
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "node_modules",
							Metadata: map[string]interface{}{
								"build": true,
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
			Expect(linkCalls[0].Oldname).To(Equal(filepath.Join("some-binding-path", ".yarnrc")))
			home, err := os.UserHomeDir()
			Expect(err).NotTo(HaveOccurred())
			Expect(linkCalls[0].Newname).To(Equal(filepath.Join(home, ".yarnrc")))
			Expect(symlinker.UnlinkCall.CallCount).To(Equal(2))
			Expect(unlinkPaths[1]).To(Equal(filepath.Join(home, ".yarnrc")))
		})
	})

	context("failure cases", func() {
		context("when the modules layer cannot be retrieved", func() {
			it.Before(func() {
				Expect(ioutil.WriteFile(filepath.Join(layersDir, "modules.toml"), nil, 0000)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("failed to parse layer content metadata:")))
				Expect(err).To(MatchError(ContainSubstring("modules.toml")))
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when npmrc service binding resolution fails", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "npmrc" {
						return nil, errors.New("npmrc binding error")
					}
					return nil, nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("npmrc binding error"))
			})
		})
		context("when yarnrc service binding resolution fails", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "yarnrc" {
						return nil, errors.New("yarnrc binding error")
					}
					return nil, nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("yarnrc binding error"))
			})
		})

		context("when npmrc service binding resolution returns more than 1 binding", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "npmrc" {
						return []servicebindings.Binding{
							servicebindings.Binding{
								Name: "first",
								Type: "npmrc",
							},
							servicebindings.Binding{
								Name: "second",
								Type: "npmrc",
							},
						}, nil
					}
					return nil, nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("binding resolver found more than one binding of type 'npmrc'"))
			})
		})
		context("when yarnrc service binding resolution returns more than 1 binding", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "yarnrc" {
						return []servicebindings.Binding{
							servicebindings.Binding{
								Name: "first",
								Type: "yarnrc",
							},
							servicebindings.Binding{
								Name: "second",
								Type: "yarnrc",
							},
						}, nil
					}
					return nil, nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("binding resolver found more than one binding of type 'yarnrc'"))
			})
		})
		context("when the 'npmrc' service binding does not contain an .npmrc entry", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "npmrc" {
						return []servicebindings.Binding{
							servicebindings.Binding{
								Name: "first",
								Type: "npmrc",
							},
						}, nil
					}
					return nil, nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("binding of type 'npmrc' does not contain required entry '.npmrc'"))
			})
		})
		context("when the 'yarnrc' service binding does not contain an .yarnrc entry", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "yarnrc" {
						return []servicebindings.Binding{
							servicebindings.Binding{
								Name: "first",
								Type: "yarnrc",
							},
						}, nil
					}
					return nil, nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("binding of type 'yarnrc' does not contain required entry '.yarnrc'"))
			})
		})

		context("when the check for the install process fails", func() {
			it.Before(func() {
				installProcess.ShouldRunCall.Stub = nil
				installProcess.ShouldRunCall.Returns.Err = errors.New("failed to determine if process should run")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("failed to determine if process should run"))
			})
		})

		context("when the install process cannot be executed", func() {
			it.Before(func() {
				installProcess.ExecuteCall.Returns.Error = errors.New("failed to execute install process")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("failed to execute install process"))
			})
		})

		context("when install is skipped and node_modules cannot be removed", func() {
			it.Before(func() {
				installProcess.ShouldRunCall.Stub = nil
				installProcess.ShouldRunCall.Returns.Run = false
				Expect(os.Chmod(filepath.Join(workingDir), 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(filepath.Join(workingDir), os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when install is skipped and symlinking node_modules fails", func() {
			it.Before(func() {
				installProcess.ShouldRunCall.Stub = nil
				installProcess.ShouldRunCall.Returns.Run = false
				symlinker.LinkCall.Stub = nil
				symlinker.LinkCall.Returns.Error = errors.New("some symlinking error")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(symlinker.LinkCall.CallCount).To(Equal(1))
				Expect(err).To(MatchError(ContainSubstring("some symlinking error")))
			})
		})

		context("when .npmrc service binding symlink cannot be created", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "npmrc" {
						return []servicebindings.Binding{
							servicebindings.Binding{
								Name: "first",
								Type: "npmrc",
								Path: "some-binding-path",
								Entries: map[string]*servicebindings.Entry{
									".npmrc": servicebindings.NewEntry("some-path"),
								},
							},
						}, nil
					}
					return nil, nil
				}
				symlinker.LinkCall.Stub = func(o string, n string) error {
					if strings.Contains(o, ".npmrc") {
						return errors.New("symlinking .npmrc error")
					}
					return nil
				}
			})

			it("errors", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("symlinking .npmrc error")))
			})
		})
		context("when .yarnrc service binding symlink cannot be created", func() {
			it.Before(func() {
				bindingResolver.ResolveCall.Stub = func(typ, provider, platform string) ([]servicebindings.Binding, error) {
					if typ == "yarnrc" {
						return []servicebindings.Binding{
							servicebindings.Binding{
								Name: "first",
								Type: "yarnrc",
								Path: "some-binding-path",
								Entries: map[string]*servicebindings.Entry{
									".yarnrc": servicebindings.NewEntry("some-path"),
								},
							},
						}, nil
					}
					return nil, nil
				}
				symlinker.LinkCall.Stub = func(o string, n string) error {
					if strings.Contains(o, ".yarnrc") {
						return errors.New("symlinking .yarnrc error")
					}
					return nil
				}
			})

			it("errors", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring("symlinking .yarnrc error")))
			})
		})

		context("when the layers directory cannot be written to", func() {
			it.Before(func() {
				Expect(os.Chmod(layersDir, 4444)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
					Layers: packit.Layers{Path: layersDir},
				})
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when the path parser returns an error", func() {
			it.Before(func() {
				pathParser.GetCall.Returns.Err = errors.New("path-parser-error")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("path-parser-error"))
			})
		})
		context("when .npmrc binding symlink can't be cleaned up", func() {
			it.Before(func() {
				symlinker.UnlinkCall.Stub = func(p string) error {
					if strings.Contains(p, ".npmrc") {
						return errors.New("unlinking .npmrc error")
					}
					return nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("unlinking .npmrc error"))
			})
		})
		context("when .yarnrc binding symlink can't be cleaned up", func() {
			it.Before(func() {
				symlinker.UnlinkCall.Stub = func(p string) error {
					if strings.Contains(p, ".yarnrc") {
						return errors.New("unlinking .yarnrc error")
					}
					return nil
				}
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					CNBPath:    cnbDir,
					Layers:     packit.Layers{Path: layersDir},
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError("unlinking .yarnrc error"))
			})
		})
	})
}
