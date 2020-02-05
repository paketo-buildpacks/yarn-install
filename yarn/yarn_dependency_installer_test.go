package yarn_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/yarn-cnb/yarn"
	"github.com/cloudfoundry/yarn-cnb/yarn/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testYarnDependencyInstaller(t *testing.T, context spec.G, it spec.S) {
	var Expect = NewWithT(t).Expect

	context("Install", func() {
		var (
			cnbPath     string
			destination string
			shasum      string
			pathEnv     string
			transport   *fakes.Transport

			installer yarn.YarnDependencyInstaller
		)

		it.Before(func() {
			var err error
			destination, err = ioutil.TempDir("", "destination")
			Expect(err).NotTo(HaveOccurred())

			cnbPath, err = ioutil.TempDir("", "cnb-path")
			Expect(err).NotTo(HaveOccurred())

			pathEnv = os.Getenv("PATH")
			Expect(os.Setenv("PATH", "/some/bin")).To(Succeed())

			buffer := bytes.NewBuffer(nil)
			hash := sha256.New()

			writer := io.MultiWriter(buffer, hash)
			gw := gzip.NewWriter(writer)
			tw := tar.NewWriter(gw)

			err = tw.WriteHeader(&tar.Header{
				Name: "some-file",
				Mode: 0644,
				Size: int64(len("some-contents")),
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = tw.Write([]byte("some-contents"))
			Expect(err).NotTo(HaveOccurred())

			Expect(tw.Close()).To(Succeed())
			Expect(gw.Close()).To(Succeed())

			sum := hash.Sum(nil)
			shasum = hex.EncodeToString(sum[:])

			transport = &fakes.Transport{}
			transport.DropCall.Returns.ReadCloser = ioutil.NopCloser(buffer)

			installer = yarn.NewYarnDependencyInstaller(transport)
		})

		it.After(func() {
			Expect(os.Setenv("PATH", pathEnv)).To(Succeed())

			Expect(os.RemoveAll(destination)).To(Succeed())
			Expect(os.RemoveAll(cnbPath)).To(Succeed())
		})

		it("installs yarn into the given location", func() {
			err := installer.Install([]yarn.BuildpackMetadataDependency{
				{
					URI:     "some-uri",
					SHA256:  "some-sha",
					Version: "1.2.3",
				},
				{
					URI:     "other-uri",
					SHA256:  shasum,
					Version: "4.5.6",
				},
			}, cnbPath, destination)
			Expect(err).NotTo(HaveOccurred())

			contents, err := ioutil.ReadFile(filepath.Join(destination, "some-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("some-contents"))

			Expect(transport.DropCall.Receives.Root).To(Equal(cnbPath))
			Expect(transport.DropCall.Receives.Uri).To(Equal("other-uri"))

			Expect(os.Getenv("PATH")).To(Equal(fmt.Sprintf("/some/bin:%s", filepath.Join(destination, "bin"))))
		})

		context("failure cases", func() {
			context("when the transport fails", func() {
				it.Before(func() {
					transport.DropCall.Returns.Error = errors.New("failed to download dependency")
				})

				it("returns an error", func() {
					err := installer.Install([]yarn.BuildpackMetadataDependency{
						{
							URI:     "some-uri",
							SHA256:  shasum,
							Version: "1.2.3",
						},
					}, cnbPath, destination)
					Expect(err).To(MatchError("failed to install yarn: failed to download dependency"))
				})
			})

			context("when the download is not a valid tarball", func() {
				it.Before(func() {
					transport.DropCall.Returns.ReadCloser = ioutil.NopCloser(strings.NewReader("not a valid tgz"))
				})

				it("returns an error", func() {
					err := installer.Install([]yarn.BuildpackMetadataDependency{
						{
							URI:     "some-uri",
							SHA256:  shasum,
							Version: "1.2.3",
						},
					}, cnbPath, destination)
					Expect(err).To(MatchError(ContainSubstring("failed to install yarn:")))
					Expect(err).To(MatchError(ContainSubstring("invalid header")))
				})
			})

			context("when the download checksum does not match", func() {
				it("returns an error", func() {
					err := installer.Install([]yarn.BuildpackMetadataDependency{
						{
							URI:     "some-uri",
							SHA256:  "some-does-not-match",
							Version: "1.2.3",
						},
					}, cnbPath, destination)
					Expect(err).To(MatchError(ContainSubstring("failed to install yarn:")))
					Expect(err).To(MatchError(ContainSubstring("checksum does not match")))
				})
			})
		})
	})
}
