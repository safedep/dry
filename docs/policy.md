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
// err wraps project.ErrDecode or project.ErrHeader: reject the whole document.

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
- When two rules of one effect match, the rule with the lower id decides.
- `Normalize` drops a rule with an unknown effect, and a trust entry with no id, no known scope, a
  prefix selector or a `vers` range that does not parse. It gives a block entry with no id the id `-`.
- `NewIndex` also drops a rule whose selector the identity cannot parse, and a trust entry with a
  version in the PURL and a `vers` range. It returns each dropped rule with the reason.
- A block entry whose `vers` range does not parse, or that has both a version in the PURL and a range,
  matches every version of its packages. A block that a tool drops is a block that does not apply.
- When the version of a package cannot be checked, a block entry matches and a trust entry does not.
  This covers a package with no version, a version that the ecosystem cannot order, and a range whose
  bounds are not in version order.
- A version in the selector PURL matches the canonical version of the package.

### Index

`Index` holds a map from the NameKey of each exact selector to its rules, and a map from each prefix
key to its rules. A check reads one entry of the first map, and one entry of the second map for each
distinct prefix length. So a check never reads every rule, and it needs no ecosystem list.

A prefix ends with `/*` or `.*`. `pb.NameKeyPrefix` reads the boundary from the raw PURL before it
decodes it, then folds the part before the `*` with the identity rule of the ecosystem. So
`pkg:npm/@acme/*` does not select `@acmex/x`, `pkg:maven/org.apache.*` does not select
`org.apachex`, and `pkg:pypi/foo.*` selects `foo-bar` and `foo_bar`.

### `vers`

`ParseVers` follows the `vers` specification of the package-url project: `|` separates constraints,
white space is not significant, the comparators are `=`, `!=`, `<`, `<=`, `>` and `>=`, a constraint
with no comparator means `=`, and `*` alone selects every version. The scheme must equal the PURL type
of the rule.

- `ParseVers` needs no version order. It checks that each version is unique and that the `<`, `<=`,
  `>` and `>=` constraints alternate.
- `Range.Contains` checks the version order of the bounds for each version that it tests. Bounds that
  are not in version order give an error.
- `=` and `!=` constraints can stand at any position, because each one checks one version. A range of
  only `!=` constraints contains every other version.

### Benchmark

One check at 10,000 rules, on an Intel Xeon at 2.1 GHz:

| Case | `match` test identity | `pbidentity` | `pbidentity`, with the PURL parse |
|---|---|---|---|
| Miss | 24 ns | 16 ns | 1.2 µs |
| Prefix hit | 76 ns | 64 ns | |
| Trust hit with a range | 0.7 µs | 0.13 µs | 2.3 µs |
| Block hit with a range of 3 bounds (PyPI) | | 0.31 µs | 5.5 µs |

The target is 10 µs. A linear scan took 79 µs in a proof of concept. `TestDecideIsFast` in
`policy/pbidentity` fails when a check takes 10 µs or more. `pbidentity` parses the version of a
package once, and caches the parsed bound versions of the ranges, because a parse of a PyPI version
costs microseconds.

## `policy/project`

- `Decode` keeps unknown fields (`proto.Unmarshal`) and reports each one by its contract path, as
  `package_admission.7`. Paths outside the `PolicySpec` start with `header`, `field_sources` or
  `resolved_policy`, so they never look like a contract path.
- **Header check.** The header must be present. The tenant must equal `Expect.Tenant` when it is set.
  The subject kind must equal `Expect.SubjectKind`. For an endpoint, `header.machine_id` must equal
  `Expect.MachineID`, and the subject id is not checked. For a project, the subject id must equal
  `Expect.ProjectID`. A failure returns `ErrHeader`.
- **Package rules.** A trust entry with an unknown field, in the rule or in its selector, is dropped. A
  block entry with an unknown field is kept, and the field is reported. An unknown scope value is
  ignored. `match.Normalize` applies the other rules. An expired rule stays in `Rules`.
- **Settings.** `Values` maps each contract path to a `time.Duration`, `time.Time`, `int64`,
  `float64`, `bool`, `string`, `[]byte`, `Enum`, or `[]any` of these. `Sources` maps each field to the
  layer that set it.
- **Ordered enums.** An unknown value of an enum with the option `ordered` becomes the known value with
  the highest `strictness`. An unknown value of an enum with no order fails its capability.
- **Contract limits.** `Decode` runs protovalidate on each capability except `core`. A violation fails
  the capability: `Values` holds none of its settings, and `FailedCapabilities` names it.
  `Projection.Restore(previous)` takes a failed capability from the projection of the previous policy.
  The package rules of `core` have their own rules and never fail as a whole.
- `Fields` lists each contract field with a field contract, such as `binding_required`, from the
  descriptor. A tool test checks its binding table against it.

The contract has no `FieldExceptionReason` for a value that fails its limits, so
`ExceptionInvalidValue` maps to `FIELD_EXCEPTION_REASON_UNSPECIFIED`.

## Not in these packages

`policy/bind` (the binding table runner) and `cloud/configsync` (the fetcher and the policy cache) come
in later changes.
