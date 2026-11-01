# policy

The `policy` packages apply the cloud policy of SafeDep Cloud on a client: pmg, vet and the pull
request check. The design is the spec `docs/specs/2026-10-30-pmg-cloud-policy-design.md` in
control-tower, Design 4 and 5, under the parent spec `2026-10-28-cloud-policy-management-design.md`.

| Package | Does | Imports the API contract |
|---|---|---|
| `policy/project` | Decodes a `ResolvedPolicy`, checks the header, applies the projection rules, and returns a plain value tree and the package rules | Yes |
| `policy/match` | Decides which package rule applies to a package | No |
| `policy/pbidentity` | Gives `match` the canonical package identity of `api/pb` | Through `api/pb` |

`match` has no import of the API contract, so vet, whose rules allow only `model` to import `api/pb`,
can use it with its own `match.Identity`. `TestNoContractImport` keeps this rule.

## Flow in a tool

```go
proj, err := project.Decode(raw, project.Expect{
	Tenant:      tenant, // "" with no login: skips only the tenant check
	SubjectKind: specv1.SubjectKind_SUBJECT_KIND_ENDPOINT,
	MachineID:   machineID,
})
if err != nil {
	return err // wraps project.ErrDecode or project.ErrHeader: reject the whole document
}

proj.Restore(previous) // takes each failed capability from the previous policy

rules := append(proj.Rules, localRules...)
index, dropped := match.NewIndex(rules, pbidentity.Identity{})

decision := index.Decide(match.Query{
	Package: pbidentity.Package(pv),
	Control: match.ScopeMalware,
	Verdict: match.VerdictMalware,
	Now:     time.Now(),
})
```

Report `proj.Exceptions` and `dropped` in the `ApplyReport` of the check-in. `Exception.FieldException`
gives the API form.

## `policy/match`

`Index.Decide` applies this order. The first step that decides wins.

1. A block entry matches: `EffectBlock`.
2. The verdict is `VerdictVerifiedMalware`: no rule. This is the verified malware guard. It holds for
   every scope and every control.
3. A trust entry matches and its scopes hold the control: `EffectTrust`.
4. Otherwise: no rule.

Rules:

- A rule applies while `now` is before its expiry. A zero `Query.Now` means `time.Now()`.
- A query with no control (`Control` zero) gets no trust entry. A block entry still applies.
- When two rules of one effect match, the rule with the lower id decides. The other fields of the rule
  break a tie, so the order of the input never decides.
- `Normalize` drops a rule with an unknown effect, and a trust entry with no id, no expiry
  (`ErrNoExpiry`), no known scope, a prefix selector or a `vers` range that does not parse. It gives a
  block entry with no id the id `-`. A cache that a user edits has no server check, so a trust entry
  with no expiry would never end.
- `NewIndex` also drops a rule whose selector the identity cannot parse, and a trust entry whose `vers`
  range is invalid for its selector. It returns each dropped rule with the reason.
- A `vers` range is invalid when it does not parse, when the PURL also has a version, when its scheme
  names another ecosystem than the PURL type, or when its bounds are not in version order. A block
  entry with an invalid range matches every version of its packages. A block that a tool drops is a
  block that does not apply.
- `Validate` applies every check of `NewIndex` to one rule, and also refuses a block entry with an
  invalid range. A server calls it before it stores a rule.
- When the version of a package cannot be checked, a block entry matches and a trust entry does not.
  This covers a package with no version, and a version that the ecosystem cannot order against a
  range.
- A version in the selector PURL matches the same canonical version text. A block entry also matches
  a version that the ecosystem orders equal to it, such as `1.0.0+build` for `1.0.0` under npm. A
  version that the ecosystem cannot order does not match a block entry with one version: GitHub
  Actions has no version order, and a block on `v4` must not block `v3`.

### Index

`Index` holds a map from the NameKey of each exact selector to its rules, and a map from each prefix
key to its rules. A check reads one entry of the first map, and one entry of the second map for each
distinct prefix length. So a check never reads every rule, and it needs no ecosystem list.

A prefix ends with `/*` or `.*`. `Identity.Prefix` returns its key. `pb.NameKeyPrefix` reads the
boundary from the raw PURL before it decodes it, then folds the part before the `*` with the identity
rule of the ecosystem. So `pkg:npm/@acme/*` does not select `@acmex/x`, `pkg:maven/org.apache.*` does
not select `org.apachex`, `pkg:pypi/foo.*` selects `foo-bar` and `foo_bar`, and
`pkg:nuget/Microsoft.*` selects `microsoft.evil`.

**Non-ASCII names.** NuGet ids can hold the Unicode letters of .NET `\w`. The NuGet rule folds the
ASCII letters of every id and keeps each other character as it is, so `pkg:nuget/Microsoft.*` selects
`Microsoft.Évil`. Two ids that differ only in the case of a non-ASCII letter, as `Évil` and `évil`,
have two keys, but NuGet resolves them to one package. A Unicode fold would close this gap, but it
would also merge the KELVIN SIGN with `k`, which NuGet keeps apart. The other ASCII-only folds (PyPI,
Cargo, Packagist, GitHub, GitLab, Bitbucket, Chrome) keep a non-ASCII name raw. Their registries
accept only ASCII names.

### `vers`

`ParseVers` follows the `vers` specification of the package-url project: `|` separates constraints,
white space is not significant, the comparators are `=`, `!=`, `<`, `<=`, `>` and `>=`, a constraint
with no comparator means `=`, and `*` alone selects every version. The scheme must equal the PURL type
of the rule.

- The scheme names the ecosystem of the PURL type through the aliases of the identity, so
  `pkg:pip/foo` takes `vers:pypi/...` and `pkg:golang/...` takes `vers:go/...`.
- `ParseVers` needs no version order. It checks that each version is unique and that the `<`, `<=`,
  `>` and `>=` constraints alternate.
- `ParseVers` refuses a range of only `!=` constraints. The `vers` specification gives it no version,
  and the plain reading gives it every other version. Each reading fails open for one effect. So the
  range is invalid: a block entry matches every version, a trust entry is dropped, and the server
  refuses it.
- `NewIndex` and `Validate` check that the bounds go up in version order, and that no `=` and `!=`
  constraint name one version, as `=1.0.0|!=1.0.0+build` does under npm. They build a package of the
  selector at each version through the `Identity` and compare. A prefix selector gets the name `x` in
  place of its `*`. Bounds out of order, or bounds that the ecosystem cannot order, make the range
  invalid. `Range.Contains` checks the order again for each version that it tests.
- `=` and `!=` constraints can stand at any position, because each one checks one version.
  `Range.Contains` fails when a `=` and a `!=` constraint both match the tested version, so the
  written order never decides.

### Benchmark

One check at 10,000 rules, on an Intel Xeon at 2.1 GHz:

| Case | `match` test identity | `pbidentity` | `pbidentity`, with the PURL parse |
|---|---|---|---|
| Miss | 18 ns | 17 ns | 1.4 µs |
| Prefix hit | 79 ns | 79 ns | |
| Trust hit with a range | 0.74 µs | 0.14 µs | 2.1 µs |
| Block hit with a range of 3 bounds (PyPI) | | 0.34 µs | 5.9 µs |

The target is 10 µs. A linear scan took 79 µs in a proof of concept. `BenchmarkDecide` in
`policy/match` and `policy/pbidentity` measures it. No test gates the time, because a shared CI
runner can be slow at random. `pbidentity` parses the version of a
package once, and caches the parsed bound versions of the ranges, because a parse of a PyPI version
costs microseconds.

`BenchmarkDecode` in `policy/project` measures `Decode` of a resolved policy with 10,000 package rules:
237 ms, 25 MB and 700,000 allocations, on an Intel Xeon at 2.8 GHz. The cost grows with the number of
rules, about 24 µs for each rule, so 100 rules take about 2.4 ms. Two thirds of it is protovalidate: the
CEL rules of the contract run on each package rule. A tool that decodes before each command pays this
each time.

## `policy/project`

- `Decode` keeps unknown fields (`proto.Unmarshal`) and reports each one by its contract path, as
  `package_admission.7`. Paths outside the `PolicySpec` start with `header`, `field_sources` or
  `resolved_policy`, so they never look like a contract path.
- A document with no `spec` returns `ErrDecode`.
- **Header check.** The header and its `issued_at` must be present. The tenant must equal
  `Expect.Tenant` when it is set. The comparison is case-sensitive, so a difference fails closed.
  The subject kind must equal `Expect.SubjectKind`. For an endpoint, `header.machine_id` must equal
  `Expect.MachineID`, and the subject id is not checked. For a project, the subject id must equal
  `Expect.ProjectID`. A failure returns `ErrHeader`.
- **Package rules.** `Decode` runs the protovalidate rules of each `PackageRule`.
  - A trust entry is dropped when it has an unknown field at any depth (rule, selector or
    `expires_at`), has no `expires_at`, or fails a contract limit.
  - A block entry with an unknown field or a failed contract limit is kept, and each one is reported.
    `NewIndex` drops it later only when its selector does not parse.
  - An unknown scope value is ignored. `match.Normalize` applies the other rules. An expired rule stays
    in `Rules`.
  - The list has no cap in `project`. The receive limit of the fetch, 4 MiB, bounds it.
- **Settings.** `Values` maps each contract path to a `time.Duration`, `time.Time`, `int64`,
  `float64`, `bool`, `string`, `[]byte`, `Enum`, or `[]any` of these. `Sources` maps each field to the
  layer that set it.
- **Ordered enums.** An unknown value of an enum with the option `ordered` becomes the known value with
  the highest `strictness`. An unknown value of an enum with no order fails its capability.
- **Contract limits.** `Decode` runs protovalidate on each capability except `core`. A violation fails
  the capability: `Values` and `Sources` hold none of its settings, and `FailedCapabilities` names it.
  `Projection.Restore(previous)` takes a failed capability from the projection of the previous policy.
  An enum value 0 under `not_in: [0]` fails its capability the same way. The package rules of `core`
  have their own rules, one rule at a time, and never fail as a whole.
- `Fields` lists each contract field with a field contract, such as `binding_required`, from the
  descriptor. A tool test checks its binding table against it.

The contract has no `FieldExceptionReason` for a value that fails its limits, so
`ExceptionInvalidValue` maps to `FIELD_EXCEPTION_REASON_UNSPECIFIED`.

## Not in these packages

`policy/bind` (the binding table runner) comes in a later change. `cloud/configsync` fetches the
resolved policy and keeps the policy cache. See [cloud-configsync.md](cloud-configsync.md).
