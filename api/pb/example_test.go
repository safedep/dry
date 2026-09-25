package pb_test

import (
	"fmt"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"

	"github.com/safedep/dry/api/pb"
)

// ExamplePackageVersion shows the one identity two spellings of a PyPI
// release share, the two supported comparisons, and the proto a client
// sends. The client sends the raw spelling. The server folds it under its
// own rule, so a rule change on the server needs no client release.
func ExamplePackageVersion() {
	observed := pb.NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "DataToolKit", "1.0.0")
	stored := pb.NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "datatoolkit", "1.0")

	fmt.Println(observed.Name(), observed.Version())
	fmt.Println(observed.Equal(stored))
	fmt.Println(observed.Key() == stored.Key())

	request := observed.RawProto()
	fmt.Println(request.GetPackage().GetName(), request.GetVersion())

	// Output:
	// datatoolkit 1
	// true
	// true
	// DataToolKit 1.0.0
}

// ExamplePackageVersion_versionParsed shows that VersionParsed is false for
// an ecosystem with no version rule, where the raw version is the identity,
// and is false for a PyPI version the PEP 440 grammar rejects, where the
// value matches only its own spelling.
func ExamplePackageVersion_versionParsed() {
	npm := pb.NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_NPM, "express", "4.17.1")
	fmt.Println(npm.HasVersionRule(), npm.VersionParsed())

	invalid := pb.NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "pkg", "1.0_1")
	fmt.Println(invalid.HasVersionRule(), invalid.VersionParsed(), invalid.Version())

	// Output:
	// false false
	// true false 1.0_1
}
