package yarn

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/blang/semver"
	"github.com/cloudfoundry/packit/cargo"
	"github.com/cloudfoundry/packit/vacation"
)

//go:generate faux --interface Transport --output fakes/transport.go
type Transport interface {
	Drop(root, uri string) (io.ReadCloser, error)
}

type YarnDependencyInstaller struct {
	transport Transport
}

func NewYarnDependencyInstaller(transport Transport) YarnDependencyInstaller {
	return YarnDependencyInstaller{
		transport: transport,
	}
}

func (i YarnDependencyInstaller) Install(dependencies []BuildpackMetadataDependency, cnbPath, layerPath string) error {
	sort.Slice(dependencies, func(i, j int) bool {
		iVersion := semver.MustParse(dependencies[i].Version)
		jVersion := semver.MustParse(dependencies[j].Version)

		return iVersion.GT(jVersion)
	})

	bundle, err := i.transport.Drop(cnbPath, dependencies[0].URI)
	if err != nil {
		return fmt.Errorf("failed to install yarn: %w", err)
	}
	defer bundle.Close()

	validatedReader := cargo.NewValidatedReader(bundle, dependencies[0].SHA256)

	err = vacation.NewTarGzipArchive(validatedReader).Decompress(layerPath)
	if err != nil {
		return fmt.Errorf("failed to install yarn: %w", err)
	}

	ok, err := validatedReader.Valid()
	if err != nil {
		return fmt.Errorf("failed to install yarn: %w", err)
	}

	if !ok {
		return errors.New("failed to install yarn: checksum does not match")
	}

	os.Setenv("PATH", fmt.Sprintf("%s:%s", os.Getenv("PATH"), filepath.Join(layerPath, "bin")))

	return nil
}
