package common_test

import (
	"os"
	"testing"

	"github.com/paketo-buildpacks/yarn-install/common"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testYarnrcYmlParser(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("ParseLinker", func() {
		var (
			path   string
			parser common.YarnrcYmlParser
		)

		it.Before(func() {
			file, err := os.CreateTemp("", ".yarnrc.yml")
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()

			_, err = file.WriteString(`---
nodeLinker: node-modules
`)
			Expect(err).NotTo(HaveOccurred())

			path = file.Name()

			parser = common.NewYarnrcYmlParser()
		})

		it.After(func() {
			Expect(os.RemoveAll(path)).To(Succeed())
		})

		it("parses the node engine version from a package.json file", func() {
			linker, err := parser.ParseLinker(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(linker).To(Equal("node-modules"))
		})

		context("failure cases", func() {
			context("when the .yarnrc.yml file does not exist", func() {
				it("returns an error", func() {
					_, err := parser.ParseLinker("/missing/file")
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			context("when the .yarnrc.yml contents are malformed", func() {
				it.Before(func() {
					err := os.WriteFile(path, []byte("%%%"), 0644)
					Expect(err).NotTo(HaveOccurred())
				})

				it("returns an error", func() {
					_, err := parser.ParseLinker(path)
					Expect(err).To(MatchError(ContainSubstring("could not find expected directive name")))
				})
			})
		})
	})
}
