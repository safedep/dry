# AGENTS.md

This file gives guidance to AI coding agents that work on dry.

dry is a library of Go packages that every SafeDep tool imports: vet, pmg, control-tower and
others. A change to an exported API, to the output of a function, or to `go.mod` reaches all of
them.

## Build and test

```bash
go build ./...
go test ./... -count=1                  # all packages
go test ./api/pb/ -run TestName -count=1  # one test
golangci-lint run ./<package>/...       # lefthook runs it on pre-commit
bash scripts/check-tui-discipline.sh    # the tui rules, also in CI
```

- The Go version is in `go.mod` and `.tool-versions`.
- Some tests need the network or a sandbox. `packageregistry` calls the live GitHub API. The
  sandbox end-to-end tests run only with `SANDBOX_ENABLE_E2E_TEST=true`.
- Use `testify` (`require` and `assert`) and table-driven tests.

## Generated code

- Mocks: list the interface in `.mockery.yml` and run `go tool mockery`. See `docs/mocks.md`.
- Other generated code: `go generate ./...`. Do not edit a generated file by hand.

## Rules

- **Keep the API compatible.** Add a function or a field. Do not change the behaviour or the output
  of an exported function that callers already use. Some outputs are frozen on purpose, because
  callers store them. Tests pin them, for example `TestNewPurlPackageVersionIsFrozen` in `api/pb`.
  Read the doc comment before you change a function that says it is frozen.
- **Think before you add a dependency.** A new module in `go.mod`, with the modules it requires,
  reaches every SafeDep tool. Prefer the standard library. Run `go mod tidy` and read the diff of
  `go.mod`.
- **Package docs.** A package with a design or a rule of its own has a page in `docs/`, for example
  `docs/tui.md` and `docs/localdb.md`. Update it in the same change.
- **Commits.** Use the form `type(scope): subject`, for example `feat(pb): ...` or
  `fix(tui): ...`.

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
