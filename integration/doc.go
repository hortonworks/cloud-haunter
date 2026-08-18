// Package integration holds end-to-end tests that exercise the real
// operation -> filter -> action chain against a fake cloud provider, mirroring
// the sequence main() runs but without flag parsing or live cloud access.
//
// It intentionally has no non-test code; see chain_test.go.
package integration
