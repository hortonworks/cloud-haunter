# Development

**If you are just interested in releases then check the Github releases tab.**

Otherwise, clone the repo and use `make build` to build it the same way we are building it or simply issue `go build` if you know what you are doing.

You may also need to format your changes before building to pass the pre-build checks, you can do this with `make format`

The dependencies are [vendored](https://go.dev/ref/mod#vendoring) inside the `vendor` directory.

## Dependencies

 * Golang 1.26
 * Docker (podman may work, untested)
 * GNU Make

## Adding a new dependency

 * Import the dependency in your go code
 * Run `make mod-tidy`

## Releasing a new version
```
VERSION={{new-semantic-version}} make release
```