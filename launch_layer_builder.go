package yarninstall

// if launch {
// 	layer, err := context.Layers.Get("launch-modules")
// 	if err != nil {
// 		return packit.BuildResult{}, err
// 	}

// 	logger.Process("Resolving installation process")

// 	run, sha, err := installProcess.ShouldRun(projectPath, layer.Metadata)
// 	if err != nil {
// 		return packit.BuildResult{}, err
// 	}

// 	if run {
// 		logger.Subprocess("Selected default build process: 'yarn install'")
// 		logger.Break()
// 		logger.Process("Executing launch environment install process")

// 		layer, err = layer.Reset()
// 		if err != nil {
// 			return packit.BuildResult{}, err
// 		}

// 		_, err = installProcess.SetupModules(context.WorkingDir, currentModLayer, layer.Path)
// 		if err != nil {
// 			return packit.BuildResult{}, err
// 		}

// 		duration, err := clock.Measure(func() error {
// 			return installProcess.Execute(projectPath, layer.Path, true)
// 		})
// 		if err != nil {
// 			return packit.BuildResult{}, err
// 		}

// 		logger.Action("Completed in %s", duration.Round(time.Millisecond))
// 		logger.Break()

// 		layer.Metadata = map[string]interface{}{
// 			"built_at":  clock.Now().Format(time.RFC3339Nano),
// 			"cache_sha": sha,
// 		}

// 		path := filepath.Join(layer.Path, "node_modules", ".bin")
// 		layer.LaunchEnv.Append("PATH", path, string(os.PathListSeparator))

// 		logger.EnvironmentVariables(layer)

// 		logger.GeneratingSBOM(layer.Path)
// 		var sbomContent sbom.SBOM
// 		duration, err = clock.Measure(func() error {
// 			sbomContent, err = sbomGenerator.Generate(context.WorkingDir)
// 			return err
// 		})
// 		if err != nil {
// 			return packit.BuildResult{}, err
// 		}
// 		logger.Action("Completed in %s", duration.Round(time.Millisecond))
// 		logger.Break()

// 		logger.FormattingSBOM(context.BuildpackInfo.SBOMFormats...)
// 		layer.SBOM, err = sbomContent.InFormats(context.BuildpackInfo.SBOMFormats...)
// 		if err != nil {
// 			return packit.BuildResult{}, err
// 		}

// 		execdDir := filepath.Join(layer.Path, "exec.d")
// 		err = os.MkdirAll(execdDir, os.ModePerm)
// 		if err != nil {
// 			return packit.BuildResult{}, err
// 		}

// 		err = fs.Copy(filepath.Join(context.CNBPath, "bin", "setup-symlinks"), filepath.Join(execdDir, "0-setup-symlinks"))
// 		if err != nil {
// 			return packit.BuildResult{}, err
// 		}
// 	} else {
// 		logger.Process("Reusing cached layer %s", layer.Path)
// 		if !build {
// 			err := os.RemoveAll(filepath.Join(projectPath, "node_modules"))
// 			if err != nil {
// 				return packit.BuildResult{}, err
// 			}

// 			err = symlinker.Link(filepath.Join(layer.Path, "node_modules"), filepath.Join(projectPath, "node_modules"))
// 			if err != nil {
// 				return packit.BuildResult{}, err
// 			}
// 		}
// 	}

// 	layer.Launch = true

// 	layers = append(layers, layer)
// }
