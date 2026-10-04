// Package semantic is a copy of the version parsers of
// github.com/google/osv-scalibr/semantic, release v0.5.3, under the Apache
// License 2.0 (see LICENSE). api/pb uses it to order versions as OSV does.
//
// dry copies the code and does not import osv-scalibr. The osv-scalibr
// go.mod would raise the dependencies of every dry user, and the parsers
// import the standard library only.
//
// The copy differs from the original in three ways:
//   - It holds only the parsers of the ecosystems that api/pb orders:
//     Maven, NuGet, Packagist, Pub, PyPI, RubyGems and semver.
//   - parse.go is not copied. api/pb maps each ecosystem to its parser, and
//     errors.go holds the one error value that the parsers use.
//   - utilities.go drops isASCIILetter, which only the parsers that dry does
//     not copy use.
//
// To move to a newer release:
//  1. Copy the same files from osv-scalibr/semantic at the new release,
//     with no change, and the new LICENSE.
//  2. Remove each helper that golangci-lint reports as unused.
//  3. Copy the matching testdata/*-versions*.txt files, gzipped with
//     "gzip -9 -n", to api/pb/testdata/version-order.
//  4. Change the release in this comment and in that README.
//  5. Run go test ./api/pb/.... TestVersionOrderConformance must pass every
//     pair.
//
// Update the copy only when OSV fixes an order that matters to a SafeDep
// tool. The order rules of the registries change rarely.
package semantic
