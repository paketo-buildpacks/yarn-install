package yarn

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Buildpack struct {
	API      string            `toml:"api"`
	Info     BuildpackInfo     `toml:"buildpack"`
	Metadata BuildpackMetadata `toml:"metadata"`
	Stacks   []BuildpackStack  `toml:"stacks"`
}

type BuildpackInfo struct {
	ID      string `toml:"id"`
	Name    string `toml:"name"`
	Version string `toml:"version"`
}

type BuildpackStack struct {
	ID string `toml:"id"`
}
type BuildpackMetadata struct {
	IncludeFiles    []string                         `toml:"include_files"`
	PrePackage      string                           `toml:"pre_package"`
	DefaultVersions BuildpackMetadataDefaultVersions `toml:"default_versions"`
	Dependencies    []BuildpackMetadataDependency    `toml:"dependencies"`
}

type BuildpackMetadataDefaultVersions struct {
	Yarn string `toml:"yarn"`
}

type BuildpackMetadataDependency struct {
	ID           string   `toml:"id"`
	Name         string   `toml:"name"`
	SHA256       string   `toml:"sha256"`
	Source       string   `toml:"source"`
	SourceSHA256 string   `toml:"source_sha256"`
	Stacks       []string `toml:"stacks"`
	URI          string   `toml:"uri"`
	Version      string   `toml:"version"`
}

func ParseBuildpack(path string) (Buildpack, error) {
	file, err := os.Open(path)
	if err != nil {
		return Buildpack{}, fmt.Errorf("failed to parse buildpack.toml: %w", err)
	}

	var buildpack Buildpack
	_, err = toml.DecodeReader(file, &buildpack)
	if err != nil {
		return Buildpack{}, fmt.Errorf("failed to parse buildpack.toml: %w", err)
	}

	return buildpack, nil
}
