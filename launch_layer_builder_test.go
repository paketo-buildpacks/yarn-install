package yarninstall_test

// context("when required during launch", func() {
// 	it.Before(func() {
// 		entryResolver.MergeLayerTypesCall.Returns.Launch = true
// 	})

// 	it("returns a result that installs launch modules", func() {
// 		result, err := build(packit.BuildContext{
// 			BuildpackInfo: packit.BuildpackInfo{
// 				Name:        "Some Buildpack",
// 				Version:     "1.2.3",
// 				SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 			},
// 			WorkingDir: workingDir,
// 			CNBPath:    cnbDir,
// 			Layers:     packit.Layers{Path: layersDir},
// 			Plan: packit.BuildpackPlan{
// 				Entries: []packit.BuildpackPlanEntry{
// 					{
// 						Name: "node_modules",
// 						Metadata: map[string]interface{}{
// 							"build": true,
// 						},
// 					},
// 				},
// 			},
// 			Stack: "some-stack",
// 			Platform: packit.Platform{
// 				Path: "some-platform-path",
// 			},
// 		})
// 		Expect(err).NotTo(HaveOccurred())

// 		Expect(len(result.Layers)).To(Equal(1))
// 		layer := result.Layers[0]
// 		Expect(layer.Name).To(Equal("launch-modules"))
// 		Expect(layer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 		Expect(layer.LaunchEnv).To(Equal(packit.Environment{
// 			"PATH.append": filepath.Join(layersDir, "launch-modules", "node_modules", ".bin"),
// 			"PATH.delim":  ":",
// 		}))
// 		Expect(layer.Launch).To(BeTrue())
// 		Expect(layer.Metadata).To(Equal(
// 			map[string]interface{}{
// 				"built_at":  timestamp,
// 				"cache_sha": "some-awesome-shasum",
// 			}))
// 		Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
// 			{
// 				Extension: "cdx.json",
// 				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
// 			},
// 			{
// 				Extension: "spdx.json",
// 				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
// 			},
// 			{
// 				Extension: "syft.json",
// 				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SyftFormat),
// 			},
// 		}))

// 		Expect(pathParser.GetCall.Receives.Path).To(Equal(workingDir))

// 		Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

// 		Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
// 		Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
// 		Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

// 		Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
// 		Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
// 		Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

// 		Expect(symlinker.LinkCall.CallCount).To(BeZero())

// 		Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

// 		Expect(installProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(workingDir))
// 		Expect(installProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
// 		Expect(installProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

// 		Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
// 		Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 		Expect(installProcess.ExecuteCall.Receives.Launch).To(BeTrue())

// 		Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))

// 		Expect(filepath.Join(layer.Path, "exec.d", "0-setup-symlinks")).To(BeAnExistingFile())
// 	})
// })
// context("when re-using previous launch modules layer", func() {
// 	it.Before(func() {
// 		installProcess.ShouldRunCall.Stub = nil
// 		installProcess.ShouldRunCall.Returns.Run = false
// 		entryResolver.MergeLayerTypesCall.Returns.Launch = true
// 	})

// 	it("does not redo the build process", func() {
// 		result, err := build(packit.BuildContext{
// 			BuildpackInfo: packit.BuildpackInfo{
// 				Name:        "Some Buildpack",
// 				Version:     "1.2.3",
// 				SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 			},
// 			WorkingDir: workingDir,
// 			CNBPath:    cnbDir,
// 			Layers:     packit.Layers{Path: layersDir},
// 			Plan: packit.BuildpackPlan{
// 				Entries: []packit.BuildpackPlanEntry{
// 					{
// 						Name: "node_modules",
// 						Metadata: map[string]interface{}{
// 							"build": true,
// 						},
// 					},
// 				},
// 			},
// 			Stack: "some-stack",
// 			Platform: packit.Platform{
// 				Path: "some-platform-path",
// 			},
// 		})
// 		Expect(err).NotTo(HaveOccurred())

// 		Expect(len(result.Layers)).To(Equal(1))
// 		launchLayer := result.Layers[0]
// 		Expect(launchLayer.Name).To(Equal("launch-modules"))
// 		Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
// 		Expect(launchLayer.Launch).To(BeTrue())

// 		Expect(symlinker.LinkCall.CallCount).To(Equal(1))
// 		Expect(symlinker.LinkCall.Receives.Oldname).To(Equal(filepath.Join(layersDir, "launch-modules", "node_modules")))
// 		Expect(symlinker.LinkCall.Receives.Newname).To(Equal(filepath.Join(workingDir, "some-project-dir", "node_modules")))
// 	})
// })
// context("during the launch installation process", func() {
// 	it.Before(func() {
// 		entryResolver.MergeLayerTypesCall.Returns.Launch = true
// 	})
// 	context("when the layer cannot be retrieved", func() {
// 		it.Before(func() {
// 			Expect(os.WriteFile(filepath.Join(layersDir, "launch-modules.toml"), nil, 0000)).To(Succeed())
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 			})
// 			Expect(err).To(MatchError(ContainSubstring("failed to parse layer content metadata:")))
// 			Expect(err).To(MatchError(ContainSubstring("modules.toml")))
// 			Expect(err).To(MatchError(ContainSubstring("permission denied")))
// 		})
// 	})

// 	context("when the check for the install process fails", func() {
// 		it.Before(func() {
// 			installProcess.ShouldRunCall.Stub = nil
// 			installProcess.ShouldRunCall.Returns.Err = errors.New("failed to determine if process should run")
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 			})
// 			Expect(err).To(MatchError("failed to determine if process should run"))
// 		})
// 	})

// 	context("when the layer cannot be reset", func() {
// 		it.Before(func() {
// 			Expect(os.Chmod(layersDir, 4444)).To(Succeed())
// 		})

// 		it.After(func() {
// 			Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				CNBPath: cnbDir,
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 				Layers: packit.Layers{Path: layersDir},
// 			})
// 			Expect(err).To(MatchError(ContainSubstring("permission denied")))
// 		})
// 	})

// 	context("when modules cannot be set up", func() {
// 		it.Before(func() {
// 			installProcess.SetupModulesCall.Returns.Error = errors.New("failed to setup modules")
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				CNBPath: cnbDir,
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 				Layers: packit.Layers{Path: layersDir},
// 			})
// 			Expect(err).To(MatchError("failed to setup modules"))
// 		})
// 	})

// 	context("when the install process cannot be executed", func() {
// 		it.Before(func() {
// 			installProcess.ExecuteCall.Returns.Error = errors.New("failed to execute install process")
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 			})
// 			Expect(err).To(MatchError("failed to execute install process"))
// 		})
// 	})

// 	context("when the BOM cannot be generated", func() {
// 		it.Before(func() {
// 			sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
// 		})
// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				BuildpackInfo: packit.BuildpackInfo{
// 					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
// 				},
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{{Name: "node_modules"}},
// 				},
// 				Stack: "some-stack",
// 			})
// 			Expect(err).To(MatchError("failed to generate SBOM"))
// 		})
// 	})

// 	context("when the BOM cannot be formatted", func() {
// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				BuildpackInfo: packit.BuildpackInfo{
// 					SBOMFormats: []string{"random-format"},
// 				},
// 				Layers: packit.Layers{Path: layersDir},
// 			})
// 			Expect(err).To(MatchError("\"random-format\" is not a supported SBOM format"))
// 		})
// 	})

// 	context("when exec.d setup fails", func() {
// 		it.Before(func() {
// 			installProcess.ShouldRunCall.Stub = nil
// 			installProcess.ShouldRunCall.Returns.Run = true
// 			Expect(os.RemoveAll(filepath.Join(cnbDir, "bin"))).To(Succeed())
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 			})
// 			Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
// 		})
// 	})

// 	context("when install is skipped and node_modules cannot be removed", func() {
// 		it.Before(func() {
// 			installProcess.ShouldRunCall.Stub = nil
// 			installProcess.ShouldRunCall.Returns.Run = false
// 			Expect(os.Chmod(filepath.Join(workingDir), 0000)).To(Succeed())
// 		})

// 		it.After(func() {
// 			Expect(os.Chmod(filepath.Join(workingDir), os.ModePerm)).To(Succeed())
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 			})
// 			Expect(err).To(MatchError(ContainSubstring("permission denied")))
// 		})
// 	})

// 	context("when install is skipped and symlinking node_modules fails", func() {
// 		it.Before(func() {
// 			installProcess.ShouldRunCall.Stub = nil
// 			installProcess.ShouldRunCall.Returns.Run = false
// 			symlinker.LinkCall.Stub = nil
// 			symlinker.LinkCall.Returns.Error = errors.New("some symlinking error")
// 		})

// 		it("returns an error", func() {
// 			_, err := build(packit.BuildContext{
// 				WorkingDir: workingDir,
// 				CNBPath:    cnbDir,
// 				Layers:     packit.Layers{Path: layersDir},
// 				Plan: packit.BuildpackPlan{
// 					Entries: []packit.BuildpackPlanEntry{
// 						{Name: "node_modules"},
// 					},
// 				},
// 			})
// 			Expect(symlinker.LinkCall.CallCount).To(Equal(1))
// 			Expect(err).To(MatchError(ContainSubstring("some symlinking error")))
// 		})
// 	})
// })
