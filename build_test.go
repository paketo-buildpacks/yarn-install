package yarninstall_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"

	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	type determinePathCallParams struct {
		Typ         string
		PlatformDir string
		Entry       string
	}

	type linkCallParams struct {
		Oldname string
		Newname string
	}

	var (
		Expect = NewWithT(t).Expect

		layersDir  string
		workingDir string
		homeDir    string
		cnbDir     string
		tmpDir     string

		determinePathCalls   []determinePathCallParams
		configurationManager *fakes.ConfigurationManager
		buffer               *bytes.Buffer
		entryResolver        *fakes.EntryResolver
		installProcess       *fakes.InstallProcess
		linkCalls            []linkCallParams
		sbomGenerator        *fakes.SBOMGenerator
		symlinker            *fakes.SymlinkManager
		unlinkPaths          []string
		build                packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		homeDir, err = os.MkdirTemp("", "home-dir")
		Expect(err).NotTo(HaveOccurred())

		tmpDir, err = os.MkdirTemp("", "tmp")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(workingDir, "some-project-dir"), os.ModePerm)).To(Succeed())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		installProcess = &fakes.InstallProcess{}
		installProcess.ShouldRunCall.Stub = func(string, map[string]interface{}) (bool, string, error) {
			return true, "some-awesome-shasum", nil
		}

		entryResolver = &fakes.EntryResolver{}

		buffer = bytes.NewBuffer(nil)

		t.Setenv("BP_NODE_PROJECT_PATH", "some-project-dir")

		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		configurationManager = &fakes.ConfigurationManager{}

		configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
			determinePathCalls = append(determinePathCalls, determinePathCallParams{
				Typ:         typ,
				Entry:       entry,
				PlatformDir: platform,
			})
			return "", nil
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
			entryResolver,
			configurationManager,
			homeDir,
			symlinker,
			installProcess,
			sbomGenerator,
			chronos.DefaultClock,
			scribe.NewEmitter(buffer),
			tmpDir,
		)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	context("when required during build", func() {
		it.Before(func() {
			entryResolver.MergeLayerTypesCall.Returns.Build = true
		})

		it("returns a result that installs build modules", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:        "Some Buildpack",
					Version:     "1.2.3",
					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
				},
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

			Expect(len(result.Layers)).To(Equal(1))

			layer := result.Layers[0]
			Expect(layer.Name).To(Equal("build-modules"))
			Expect(layer.Path).To(Equal(filepath.Join(layersDir, "build-modules")))
			Expect(layer.BuildEnv).To(Equal(packit.Environment{
				"PATH.append":       filepath.Join(layersDir, "build-modules", "node_modules", ".bin"),
				"PATH.delim":        ":",
				"NODE_ENV.override": "development",
			}))
			Expect(layer.Build).To(BeTrue())
			Expect(layer.Cache).To(BeTrue())
			Expect(layer.Metadata).To(Equal(
				map[string]interface{}{
					"cache_sha": "some-awesome-shasum",
				}))

			Expect(layer.SBOM.Formats()).To(HaveLen(3))

			cdx := layer.SBOM.Formats()[0]
			spdx := layer.SBOM.Formats()[1]
			syft := layer.SBOM.Formats()[2]

			Expect(cdx.Extension).To(Equal("cdx.json"))
			content, err := io.ReadAll(cdx.Content)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{
				"$schema": "http://cyclonedx.org/schema/bom-1.3.schema.json",
				"bomFormat": "CycloneDX",
				"metadata": {
					"tools": [
						{
							"name": "",
							"vendor": "anchore"
						}
					]
				},
				"specVersion": "1.3",
				"version": 1
			}`))

			Expect(spdx.Extension).To(Equal("spdx.json"))
			content, err = io.ReadAll(spdx.Content)
			Expect(err).NotTo(HaveOccurred())
			versionPattern := regexp.MustCompile(`"licenseListVersion": "\d+\.\d+"`)
			contentReplaced := versionPattern.ReplaceAllString(string(content), `"licenseListVersion": "x.x"`)

			uuidRegex := regexp.MustCompile(`[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}`)

			contentReplaced = uuidRegex.ReplaceAllString(contentReplaced, "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx")

			Expect(string(contentReplaced)).To(MatchJSON(`{
				"SPDXID": "SPDXRef-DOCUMENT",
				"creationInfo": {
					"created": "0001-01-01T00:00:00Z",
					"creators": [
						"Organization: Anchore, Inc",
						"Tool: -"
					],
					"licenseListVersion": "x.x"
				},
				"dataLicense": "CC0-1.0",
				"documentNamespace": "https://paketo.io/unknown-source-type/unknown-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
				"name": "unknown",
				"packages": [
					{
						"SPDXID": "SPDXRef-DocumentRoot-Unknown-",
						"copyrightText": "NOASSERTION",
						"downloadLocation": "NOASSERTION",
						"filesAnalyzed": false,
						"licenseConcluded": "NOASSERTION",
						"licenseDeclared": "NOASSERTION",
						"name": "",
						"supplier": "NOASSERTION"
					}
				],
				"relationships": [
					{
						"relatedSpdxElement": "SPDXRef-DocumentRoot-Unknown-",
						"relationshipType": "DESCRIBES",
						"spdxElementId": "SPDXRef-DOCUMENT"
					}
				],
				"spdxVersion": "SPDX-2.2"
			}`))

			Expect(syft.Extension).To(Equal("syft.json"))
			content, err = io.ReadAll(syft.Content)
			Expect(err).NotTo(HaveOccurred())

			versionPattern = regexp.MustCompile(`\d+\.\d+\.\d+`)

			contentReplaced = versionPattern.ReplaceAllString(string(content), `x.x.x`)

			Expect(contentReplaced).To(MatchJSON(`{
				"artifacts": [],
				"artifactRelationships": [],
				"source": {
					"id": "",
					"name": "",
					"version": "",
					"type": "",
					"metadata": null
				},
				"distro": {},
				"descriptor": {
					"name": "",
					"version": ""
				},
				"schema": {
					"version": "x.x.x",
					"url": "https://raw.githubusercontent.com/anchore/syft/main/schema/json/schema-x.x.x.json"
				}
			}`))

			Expect(len(layer.ExecD)).To(Equal(0))

			Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

			Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
			Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

			Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
			Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

			Expect(symlinker.LinkCall.CallCount).To(BeZero())

			Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

			Expect(installProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
			Expect(installProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "build-modules")))
			Expect(installProcess.ExecuteCall.Receives.Launch).To(BeFalse())

			Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))
		})
	})

	context("when required during launch", func() {
		it.Before(func() {
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
		})

		it("returns a result that installs launch modules", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:        "Some Buildpack",
					Version:     "1.2.3",
					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
				},
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

			Expect(len(result.Layers)).To(Equal(1))
			layer := result.Layers[0]
			Expect(layer.Name).To(Equal("launch-modules"))
			Expect(layer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
			Expect(layer.LaunchEnv).To(Equal(packit.Environment{
				"NODE_PROJECT_PATH.default": filepath.Join(workingDir, "some-project-dir"),
				"PATH.append":               filepath.Join(layersDir, "launch-modules", "node_modules", ".bin"),
				"PATH.delim":                ":",
			}))
			Expect(layer.Launch).To(BeTrue())
			Expect(layer.Build).To(BeFalse())
			Expect(layer.Metadata).To(Equal(
				map[string]interface{}{
					"cache_sha": "some-awesome-shasum",
				}))

			Expect(layer.SBOM.Formats()).To(HaveLen(3))

			cdx := layer.SBOM.Formats()[0]
			spdx := layer.SBOM.Formats()[1]
			syft := layer.SBOM.Formats()[2]

			Expect(cdx.Extension).To(Equal("cdx.json"))
			content, err := io.ReadAll(cdx.Content)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(MatchJSON(`{
				"$schema": "http://cyclonedx.org/schema/bom-1.3.schema.json",
				"bomFormat": "CycloneDX",
				"metadata": {
					"tools": [
						{
							"name": "",
							"vendor": "anchore"
						}
					]
				},
				"specVersion": "1.3",
				"version": 1
			}`))

			Expect(spdx.Extension).To(Equal("spdx.json"))
			content, err = io.ReadAll(spdx.Content)
			Expect(err).NotTo(HaveOccurred())

			versionPattern := regexp.MustCompile(`"licenseListVersion": "\d+\.\d+"`)
			contentReplaced := versionPattern.ReplaceAllString(string(content), `"licenseListVersion": "x.x"`)

			uuidRegex := regexp.MustCompile(`[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}`)

			contentReplaced = uuidRegex.ReplaceAllString(contentReplaced, "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx")

			Expect(string(contentReplaced)).To(MatchJSON(`{
				"SPDXID": "SPDXRef-DOCUMENT",
				"creationInfo": {
					"created": "0001-01-01T00:00:00Z",
					"creators": [
						"Organization: Anchore, Inc",
						"Tool: -"
					],
					"licenseListVersion": "x.x"
				},
				"dataLicense": "CC0-1.0",
				"documentNamespace": "https://paketo.io/unknown-source-type/unknown-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
				"name": "unknown",
				"packages": [
					{
						"SPDXID": "SPDXRef-DocumentRoot-Unknown-",
						"copyrightText": "NOASSERTION",
						"downloadLocation": "NOASSERTION",
						"filesAnalyzed": false,
						"licenseConcluded": "NOASSERTION",
						"licenseDeclared": "NOASSERTION",
						"name": "",
						"supplier": "NOASSERTION"
					}
				],
				"relationships": [
					{
						"relatedSpdxElement": "SPDXRef-DocumentRoot-Unknown-",
						"relationshipType": "DESCRIBES",
						"spdxElementId": "SPDXRef-DOCUMENT"
					}
				],
				"spdxVersion": "SPDX-2.2"
			}`))

			Expect(syft.Extension).To(Equal("syft.json"))
			content, err = io.ReadAll(syft.Content)

			versionPattern = regexp.MustCompile(`\d+\.\d+\.\d+`)

			contentReplaced = versionPattern.ReplaceAllString(string(content), `x.x.x`)

			Expect(err).NotTo(HaveOccurred())
			Expect(contentReplaced).To(MatchJSON(`{
				"artifacts": [],
				"artifactRelationships": [],
				"source": {
					"id": "",
					"name": "",
					"version": "",
					"type": "",
					"metadata": null
				},
				"distro": {},
				"descriptor": {
					"name": "",
					"version": ""
				},
				"schema": {
					"version": "x.x.x",
					"url": "https://raw.githubusercontent.com/anchore/syft/main/schema/json/schema-x.x.x.json"
				}
			}`))

			Expect(configurationManager.DeterminePathCall.CallCount).To(Equal(2))

			Expect(determinePathCalls[0].Typ).To(Equal("npmrc"))
			Expect(determinePathCalls[0].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[0].Entry).To(Equal(".npmrc"))

			Expect(determinePathCalls[1].Typ).To(Equal("yarnrc"))
			Expect(determinePathCalls[1].PlatformDir).To(Equal("some-platform-path"))
			Expect(determinePathCalls[1].Entry).To(Equal(".yarnrc"))

			Expect(symlinker.LinkCall.CallCount).To(BeZero())

			Expect(installProcess.ShouldRunCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))

			Expect(installProcess.SetupModulesCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.SetupModulesCall.Receives.CurrentModulesLayerPath).To(Equal(""))
			Expect(installProcess.SetupModulesCall.Receives.NextModulesLayerPath).To(Equal(layer.Path))

			Expect(installProcess.ExecuteCall.Receives.WorkingDir).To(Equal(filepath.Join(workingDir, "some-project-dir")))
			Expect(installProcess.ExecuteCall.Receives.ModulesLayerPath).To(Equal(filepath.Join(layersDir, "launch-modules")))
			Expect(installProcess.ExecuteCall.Receives.Launch).To(BeTrue())

			Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(workingDir))

			workspaceLink, err := os.Readlink(filepath.Join(workingDir, "some-project-dir", "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(workspaceLink).To(Equal(filepath.Join(tmpDir, "node_modules")))

			tmpLink, err := os.Readlink(filepath.Join(tmpDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpLink).To(Equal(filepath.Join(layersDir, "launch-modules", "node_modules")))
		})
	})

	context("when not required during either build or launch", func() {
		it("returns a result that has no layers", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:        "Some Buildpack",
					Version:     "1.2.3",
					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
				},
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
			Expect(result).To(Equal(packit.BuildResult{}))
		})
	})

	context("when required during both build or launch", func() {
		type setupModulesParams struct {
			WorkingDir              string
			CurrentModulesLayerPath string
			NextModulesLayerPath    string
			TempDir                 string
		}

		var setupModulesCalls []setupModulesParams

		it.Before(func() {
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
			entryResolver.MergeLayerTypesCall.Returns.Build = true
			t.Setenv("BP_NODE_PROJECT_PATH", "")

			installProcess.SetupModulesCall.Stub = func(w string, c string, n string) (string, error) {
				setupModulesCalls = append(setupModulesCalls, setupModulesParams{
					WorkingDir:              w,
					CurrentModulesLayerPath: c,
					NextModulesLayerPath:    n,
				})
				return n, nil
			}
		})
		it("returns a result that has both layers and the module setup updates accordingly", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:        "Some Buildpack",
					Version:     "1.2.3",
					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
				},
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

			launchLayer := result.Layers[1]
			Expect(launchLayer.ExecD).To(Equal([]string{filepath.Join(cnbDir, "bin", "setup-symlinks")}))
			Expect(len(result.Layers)).To(Equal(2))

			Expect(installProcess.SetupModulesCall.CallCount).To(Equal(2))

			Expect(setupModulesCalls[0].WorkingDir).To(Equal(workingDir))
			Expect(setupModulesCalls[0].CurrentModulesLayerPath).To(Equal(""))
			Expect(setupModulesCalls[0].NextModulesLayerPath).To(Equal(result.Layers[0].Path))

			Expect(setupModulesCalls[1].WorkingDir).To(Equal(workingDir))
			Expect(setupModulesCalls[1].CurrentModulesLayerPath).To(Equal(result.Layers[0].Path))
			Expect(setupModulesCalls[1].NextModulesLayerPath).To(Equal(result.Layers[1].Path))

			workspaceLink, err := os.Readlink(filepath.Join(workingDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(workspaceLink).To(Equal(filepath.Join(tmpDir, "node_modules")))

			tmpLink, err := os.Readlink(filepath.Join(tmpDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpLink).To(Equal(filepath.Join(layersDir, "build-modules", "node_modules")))
		})
	})

	context("when re-using previous modules layer", func() {
		it.Before(func() {
			installProcess.ShouldRunCall.Stub = nil
			installProcess.ShouldRunCall.Returns.Run = false
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
			entryResolver.MergeLayerTypesCall.Returns.Build = true
		})

		it("does not redo the build process", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:        "Some Buildpack",
					Version:     "1.2.3",
					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
				},
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

			Expect(len(result.Layers)).To(Equal(2))
			buildLayer := result.Layers[0]
			Expect(buildLayer.Name).To(Equal("build-modules"))
			Expect(buildLayer.Path).To(Equal(filepath.Join(layersDir, "build-modules")))
			Expect(buildLayer.Build).To(BeTrue())
			Expect(buildLayer.Cache).To(BeTrue())

			launchLayer := result.Layers[1]
			Expect(launchLayer.Name).To(Equal("launch-modules"))
			Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
			Expect(launchLayer.Launch).To(BeTrue())

			workspaceLink, err := os.Readlink(filepath.Join(workingDir, "some-project-dir", "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(workspaceLink).To(Equal(filepath.Join(tmpDir, "node_modules")))

			tmpLink, err := os.Readlink(filepath.Join(tmpDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpLink).To(Equal(filepath.Join(layersDir, "build-modules", "node_modules")))

		})
	})

	context("when re-using previous launch modules layer", func() {
		it.Before(func() {
			installProcess.ShouldRunCall.Stub = nil
			installProcess.ShouldRunCall.Returns.Run = false
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
		})

		it("does not redo the build process", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:        "Some Buildpack",
					Version:     "1.2.3",
					SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
				},
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

			Expect(len(result.Layers)).To(Equal(1))
			launchLayer := result.Layers[0]
			Expect(launchLayer.Name).To(Equal("launch-modules"))
			Expect(launchLayer.Path).To(Equal(filepath.Join(layersDir, "launch-modules")))
			Expect(launchLayer.Launch).To(BeTrue())

			workspaceLink, err := os.Readlink(filepath.Join(workingDir, "some-project-dir", "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(workspaceLink).To(Equal(filepath.Join(tmpDir, "node_modules")))

			tmpLink, err := os.Readlink(filepath.Join(tmpDir, "node_modules"))
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpLink).To(Equal(filepath.Join(layersDir, "launch-modules", "node_modules")))

		})
	})

	context("failure cases", func() {

		context("when the project path parser provided fails", func() {
			it.Before(func() {
				t.Setenv("BP_NODE_PROJECT_PATH", "does_not_exist")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					WorkingDir: workingDir,
					Layers:     packit.Layers{Path: layersDir},
					CNBPath:    cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{Name: "node_modules"},
						},
					},
				})
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("could not find project path \"%s/does_not_exist\": stat %s/does_not_exist: no such file or directory", workingDir, workingDir))))
			})
		})

		context("when determining the path for the npmrc fails", func() {
			it.Before(func() {
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "npmrc" {
						return "", errors.New("failed to determine path for npmrc")
					}
					return "", nil
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
				Expect(err).To(MatchError("failed to determine path for npmrc"))
			})
		})

		context("when determining the path for the yarnrc fails", func() {
			it.Before(func() {
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "yarnrc" {
						return "", errors.New("failed to determine path for yarnrc")
					}
					return "", nil
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
				Expect(err).To(MatchError("failed to determine path for yarnrc"))
			})
		})

		context("when .npmrc service binding symlink cannot be created", func() {
			it.Before(func() {
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "npmrc" {
						return "some-path/.npmrc", nil
					}
					return "", nil
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
				configurationManager.DeterminePathCall.Stub = func(typ, platform, entry string) (string, error) {
					if typ == "yarnrc" {
						return "some-path/.yarnrc", nil
					}
					return "", nil
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

		context("during the build installation process", func() {
			it.Before(func() {
				entryResolver.MergeLayerTypesCall.Returns.Build = true
			})
			context("when the layer cannot be retrieved", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(layersDir, "build-modules.toml"), nil, 0000)).To(Succeed())
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

			context("when the layer cannot be reset", func() {
				it.Before(func() {
					Expect(os.Chmod(layersDir, 4444)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath:    cnbDir,
						WorkingDir: workingDir,
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

			context("when modules cannot be set up", func() {
				it.Before(func() {
					installProcess.SetupModulesCall.Returns.Error = errors.New("failed to setup modules")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath:    cnbDir,
						WorkingDir: workingDir,
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("failed to setup modules"))
				})
			})

			context("when the build install process cannot be executed", func() {
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

			context("when the BOM cannot be generated", func() {
				it.Before(func() {
					sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
				})
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
						},
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{{Name: "node_modules"}},
						},
						Stack: "some-stack",
					})
					Expect(err).To(MatchError("failed to generate SBOM"))
				})
			})

			context("when the BOM cannot be formatted", func() {
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("unsupported SBOM format: 'random-format'"))
				})
			})

			context("when BP_DISABLE_SBOM is set incorrectly", func() {
				it.Before(func() {
					os.Setenv("BP_DISABLE_SBOM", "not-a-bool")
				})

				it.After(func() {
					os.Unsetenv("BP_DISABLE_SBOM")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to parse BP_DISABLE_SBOM")))
				})
			})

		})

		context("during the launch installation process", func() {
			it.Before(func() {
				entryResolver.MergeLayerTypesCall.Returns.Launch = true
			})
			context("when the layer cannot be retrieved", func() {
				it.Before(func() {
					Expect(os.WriteFile(filepath.Join(layersDir, "launch-modules.toml"), nil, 0000)).To(Succeed())
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

			context("when the layer cannot be reset", func() {
				it.Before(func() {
					Expect(os.Chmod(layersDir, 4444)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath:    cnbDir,
						WorkingDir: workingDir,
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

			context("when modules cannot be set up", func() {
				it.Before(func() {
					installProcess.SetupModulesCall.Returns.Error = errors.New("failed to setup modules")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						CNBPath:    cnbDir,
						WorkingDir: workingDir,
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{
								{Name: "node_modules"},
							},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("failed to setup modules"))
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

			context("when the BOM cannot be generated", func() {
				it.Before(func() {
					sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
				})
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"application/vnd.cyclonedx+json", "application/spdx+json", "application/vnd.syft+json"},
						},
						WorkingDir: workingDir,
						CNBPath:    cnbDir,
						Layers:     packit.Layers{Path: layersDir},
						Plan: packit.BuildpackPlan{
							Entries: []packit.BuildpackPlanEntry{{Name: "node_modules"}},
						},
						Stack: "some-stack",
					})
					Expect(err).To(MatchError("failed to generate SBOM"))
				})
			})

			context("when the BOM cannot be formatted", func() {
				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError("unsupported SBOM format: 'random-format'"))
				})
			})

			context("when BP_DISABLE_SBOM is set incorrectly", func() {
				it.Before(func() {
					os.Setenv("BP_DISABLE_SBOM", "not-a-bool")
				})

				it.After(func() {
					os.Unsetenv("BP_DISABLE_SBOM")
				})

				it("returns an error", func() {
					_, err := build(packit.BuildContext{
						WorkingDir: workingDir,
						BuildpackInfo: packit.BuildpackInfo{
							SBOMFormats: []string{"random-format"},
						},
						Layers: packit.Layers{Path: layersDir},
					})
					Expect(err).To(MatchError(ContainSubstring("failed to parse BP_DISABLE_SBOM")))
				})
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
