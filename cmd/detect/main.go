package main

import (
	"github.com/cloudfoundry/packit"
	"github.com/paketo-buildpacks/yarn-install/yarn"
)

func main() {
	packageJSONParser := yarn.NewPackageJSONParser()

	packit.Detect(yarn.Detect(packageJSONParser))
}
