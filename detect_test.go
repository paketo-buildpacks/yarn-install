package yarninstall_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
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
		yarnrcYmlParser   *fakes.YarnrcYmlParser
		workingDir        string
		detect            packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(workingDir, "custom"), os.ModePerm)).To(Succeed())

		err = os.WriteFile(filepath.Join(workingDir, "custom", ".yarnrc.yml"), []byte{}, 0644)
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(filepath.Join(workingDir, "custom", "yarn.lock"), []byte{}, 0644)
		Expect(err).NotTo(HaveOccurred())

		versionParser = &fakes.VersionParser{}
		versionParser.ParseVersionCall.Returns.Version = "some-version"

		yarnrcYmlParser = &fakes.YarnrcYmlParser{}
		yarnrcYmlParser.ParseLinkerCall.Returns.NodeLinker = "node-modules"

		projectPathParser = &fakes.PathParser{}
		projectPathParser.GetCall.Returns.ProjectPath = filepath.Join(workingDir, "custom")

		detect = yarninstall.Detect(projectPathParser, versionParser, yarnrcYmlParser)
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
		Expect(yarnrcYmlParser.ParseLinkerCall.Receives.Path).To(Equal(filepath.Join(workingDir, "custom", ".yarnrc.yml")))
	})
}
