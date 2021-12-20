package yarninstall_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/paketo-buildpacks/yarn-install/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		versionParser     *fakes.VersionParser
		projectPathParser *fakes.PathParser
		workingDir        string
		detect            packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = ioutil.TempDir("", "working-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(workingDir, "custom"), os.ModePerm)).To(Succeed())

		err = ioutil.WriteFile(filepath.Join(workingDir, "custom", "yarn.lock"), []byte{}, 0644)
		Expect(err).NotTo(HaveOccurred())

		versionParser = &fakes.VersionParser{}
		versionParser.ParseVersionCall.Returns.Version = "some-version"

		projectPathParser = &fakes.PathParser{}
		projectPathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "custom")

		detect = yarninstall.Detect(projectPathParser, versionParser)
	})

	it("returns a plan that provides node_modules and requires node and yarn", func() {
		result, err := detect(packit.DetectContext{
			WorkingDir: workingDir,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Plan).To(Equal(packit.BuildPlan{
			Provides: []packit.BuildPlanProvision{
				{Name: "node_modules"},
			},
			Requires: []packit.BuildPlanRequirement{
				{
					Name: "node",
					Metadata: yarninstall.BuildPlanMetadata{
						Version:       "some-version",
						VersionSource: "package.json",
						Build:         true,
					},
				},
				{
					Name: "yarn",
					Metadata: yarninstall.BuildPlanMetadata{
						Build: true,
					},
				},
			},
		}))

		Expect(projectPathParser.GetCall.Receives.Path).To(Equal(filepath.Join(workingDir)))
		Expect(versionParser.ParseVersionCall.Receives.Path).To(Equal(filepath.Join(workingDir, "custom", "package.json")))
	})

	context("when the node version is not in the package.json file", func() {
		it.Before(func() {
			versionParser.ParseVersionCall.Returns.Version = ""
		})

		it("returns a plan that provides node_modules", func() {
			result, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Plan).To(Equal(packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: "node_modules"},
				},
				Requires: []packit.BuildPlanRequirement{
					{
						Name: "node",
						Metadata: yarninstall.BuildPlanMetadata{
							Build: true,
						},
					},
					{
						Name: "yarn",
						Metadata: yarninstall.BuildPlanMetadata{
							Build: true,
						},
					},
				},
			}))

			Expect(projectPathParser.GetCall.Receives.Path).To(Equal(filepath.Join(workingDir)))
			Expect(versionParser.ParseVersionCall.Receives.Path).To(Equal(filepath.Join(workingDir, "custom", "package.json")))
		})
	})

	context("when there is no yarn.lock file", func() {
		it.Before(func() {
			Expect(os.Remove(filepath.Join(workingDir, "custom", "yarn.lock"))).To(Succeed())
		})

		it("fails detection", func() {
			_, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).To(MatchError(packit.Fail))
		})
	})

	context("when there is no package.json file", func() {
		it.Before(func() {
			_, err := os.Stat("/no/such/package.json")
			versionParser.ParseVersionCall.Returns.Err = err
		})

		it("fails detection", func() {
			_, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).To(MatchError(packit.Fail))
		})
	})

	context("failure cases", func() {
		context("when the yarn.lock cannot be read", func() {
			it.Before(func() {
				Expect(os.Chmod(workingDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := detect(packit.DetectContext{
					WorkingDir: workingDir,
				})
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when the package.json cannot be read", func() {
			it.Before(func() {
				versionParser.ParseVersionCall.Returns.Err = errors.New("failed to read package.json")
			})

			it("returns an error", func() {
				_, err := detect(packit.DetectContext{
					WorkingDir: workingDir,
				})
				Expect(err).To(MatchError("failed to read package.json"))
			})
		})

		context("when the project path cannot be found", func() {
			it.Before(func() {
				projectPathParser.GetCall.Returns.Err = errors.New("couldn't find directory")
			})

			it("returns an error", func() {
				_, err := detect(packit.DetectContext{
					WorkingDir: workingDir,
				})
				Expect(err).To(MatchError("couldn't find directory"))
			})
		})
	})
}
