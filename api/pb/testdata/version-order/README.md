# Version order fixtures

These files come from `semantic/testdata` of
[google/osv-scalibr](https://github.com/google/osv-scalibr) v0.5.3, under the Apache License 2.0.
The `-generated` files hold version pairs that OSV took from the registries. Each line is
`<a> <op> <b>`, where `<op>` is `<`, `=` or `>`.

`TestVersionOrderConformance` runs every pair through `CompareVersions`. dry uses the OSV order,
because the affected ranges of the vulnerability data come from OSV. `api/pb/internal/semantic`
holds a copy of the osv-scalibr parsers from the same release. Update the parsers and these files
together.
