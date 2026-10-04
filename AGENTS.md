# AGENTS.md

This file gives guidance to AI coding agents that work on dry.

## Copied third-party code

`api/pb/internal/semantic` is a copy of the version parsers of
[google/osv-scalibr](https://github.com/google/osv-scalibr) `semantic`. `api/pb` uses it to order
package versions as OSV does. dry copies the code and does not import osv-scalibr, because its
`go.mod` would raise the dependencies of every dry user.

- Do not edit the copied files. Fix an order bug upstream, then copy the new release.
- The package doc (`api/pb/internal/semantic/doc.go`) lists the differences from the original and
  the steps to move to a newer release.
- `TestVersionOrderConformance` runs the OSV test data in `api/pb/testdata/version-order`. It must
  pass every pair.
