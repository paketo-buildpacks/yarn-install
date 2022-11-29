package common_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/yarn-install/common"
	"github.com/sclevine/spec"
)

func testSymlinker(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir string
		symlinker  common.Symlinker
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		symlinker = common.NewSymlinker()
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	context("Link", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "oldname"), []byte("Hello world"), os.ModePerm)).To(Succeed())
		})
		it("creates a symlink from oldname to newname", func() {
			Expect(symlinker.Link(filepath.Join(workingDir, "oldname"), filepath.Join(workingDir, "newname"))).To(Succeed())

			fi, err := os.Lstat(filepath.Join(workingDir, "newname"))
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.Mode() & os.ModeSymlink).ToNot(BeZero())

			link, err := os.Readlink(filepath.Join(workingDir, "newname"))
			Expect(err).NotTo(HaveOccurred())
			Expect(link).To(Equal(filepath.Join(workingDir, "oldname")))
		})

		context("failure cases", func() {
			context("when the symlink cannot be created", func() {
				it("errors", func() {
					err := symlinker.Link(filepath.Join(workingDir, "oldname"), filepath.Join(workingDir, "oldname"))
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("file exists"))
				})
			})
		})
	})

	context("Unlink", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(workingDir, "oldname"), []byte("Hello world"), os.ModePerm)).To(Succeed())
			Expect(os.Symlink(filepath.Join(workingDir, "oldname"), filepath.Join(workingDir, "newname"))).To(Succeed())
		})

		it("removes the symlink", func() {
			Expect(symlinker.Unlink(filepath.Join(workingDir, "newname"))).To(Succeed())

			_, err := os.Lstat(filepath.Join(workingDir, "newname"))
			Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())

			_, err = os.Lstat(filepath.Join(workingDir, "oldname"))
			Expect(err).NotTo(HaveOccurred())
		})
		context("when the provided file does not exist", func() {
			it("is a no op", func() {
				err := symlinker.Unlink(filepath.Join(workingDir, "othername"))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		context("failure cases", func() {
			context("when the provided file is not a symlink", func() {
				it("errors", func() {
					err := symlinker.Unlink(filepath.Join(workingDir, "oldname"))
					Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("cannot unlink %s because it is not a symlink", filepath.Join(workingDir, "oldname")))))
				})
			})
		})
	})
}
