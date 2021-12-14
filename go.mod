module github.com/paketo-buildpacks/yarn-install

go 1.16

require (
	github.com/BurntSushi/toml v0.4.1
	github.com/onsi/gomega v1.17.0
	github.com/paketo-buildpacks/occam v0.2.1
	github.com/paketo-buildpacks/packit/v2 v2.0.1
	github.com/sclevine/spec v1.4.0
)

// TODO: Remove this when v0.32.0 comes out with CycloneDX JSON support
replace github.com/anchore/syft => github.com/anchore/syft v0.31.1-0.20211204010623-5374a1dc6ff6
