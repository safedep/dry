package pb

import "strings"

// cargoIdentityName is the name fold of the Cargo rule. crates.io keeps a
// crate name unique by canon_crate_name, replace(lower(name), '-', '_'), with
// a unique index on it since 2015. So two crates never differ only in case or
// in - and _, and serde-json and serde_json name one crate. Crate names are
// ASCII, so any other name stays raw and matches only itself.
func cargoIdentityName(name string) string {
	if !isASCII(name) {
		return name
	}
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}
