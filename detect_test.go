package yarninstall_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	yarninstall "github.com/paketo-buildpacks/yarn-install"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		filePath	  string
		workingDir        string
		detect            packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(workingDir, "custom"), os.ModePerm)).To(Succeed())

		err = os.WriteFile(filepath.Join(workingDir, "custom", "yarn.lock"), []byte{}, 0644)
		Expect(err).NotTo(HaveOccurred())

		filePath = filepath.Join(workingDir, "custom", "package.json")
		Expect(os.WriteFile(filePath, []byte(`{
			"engines": {
				"node": "some-version"
			}
		}`), 0600)).To(Succeed())

		t.Setenv("BP_NODE_PROJECT_PATH", "custom")

		detect = yarninstall.Detect()
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

	})

	context("when the node version is not in the package.json file", func() {
		it.Before(func() {
			Expect(os.WriteFile(filePath, []byte(`{
			}`), 0600)).To(Succeed())
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
			Expect(err).To(MatchError(packit.Fail.WithMessage("no 'yarn.lock' file found in the project path %s", filepath.Join(workingDir, "custom"))))
		})
	})

	context("when there is no package.json file", func() {
		it.Before(func() {
			Expect(os.Remove(filePath)).To(Succeed())
		})

		it("fails detection", func() {
			_, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).To(MatchError(packit.Fail.WithMessage("no 'package.json' found in project path %s", filepath.Join(workingDir, "custom"))))
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
				Expect(os.Chmod(filePath, 0000)).To(Succeed())
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

		context("when the project path cannot be found", func() {
			it.Before(func() {
				t.Setenv("BP_NODE_PROJECT_PATH", "does_not_exist")
			})

			it("returns an error", func() {
				_, err := detect(packit.DetectContext{
					WorkingDir: "/working-dir",
				})
				Expect(err).To(MatchError("could not find project path \"/working-dir/does_not_exist\": stat /working-dir/does_not_exist: no such file or directory"))
			})
		})
	})
}
