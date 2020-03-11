# Yarn Install Cloud Native Buildpack

## Integration

The Yarn Install CNB provides node_modules as a dependency. Downstream
buildpacks can require the node dependency by generating a [Build Plan
TOML](https://github.com/buildpacks/spec/blob/master/buildpack.md#build-plan-toml)
file that looks like the following:

```toml
[[requires]]

  # The name of the Yarn Install dependency is "node_modules". This value is
  # considered # part of the public API for the buildpack and will not change
  # without a plan # for deprecation.
  name = "node_modules"

  # Note: The version field is unsupported as there is no version for a set of
  # node_modules.

  # The Yarn Install buildpack does not support any non-required metadata options.
```

## Usage

To package this buildpack for consumption:

```
$ ./scripts/package.sh
```

This builds the buildpack's Go source using `GOOS=linux` by default. You can
supply another value as the first argument to `package.sh`.



