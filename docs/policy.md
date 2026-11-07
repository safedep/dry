# policy

The `policy` packages apply the cloud policy of SafeDep Cloud on a client: pmg, vet and the pull
request check. The design is the spec `docs/specs/2026-10-30-pmg-cloud-policy-design.md` in
control-tower, Design 4 and 5, under the parent spec `2026-10-28-cloud-policy-management-design.md`.

| Package | Does | Imports the API contract |
|---|---|---|
| `policy/project` | Decodes a `ResolvedPolicy`, checks the header, applies the projection rules, and returns a plain value tree and the package rules | Yes |
| `policy/match` | Decides which package rule applies to a package | No |
| `policy/pbidentity` | Gives `match` the canonical package identity of `api/pb` | Through `api/pb` |
| `policy/conformance` | Holds the shared corpus of package rule decisions, and runs it through the loader of a tool | Yes |
| `policy/spdxlicense` | Parses SPDX license expressions and applies the license gate of a policy | No |

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
- **Named vulnerabilities.** A trust entry with `ScopeNamedVulnerability` waives only the
  vulnerabilities in `Rule.Vulnerabilities`. It applies to a query with the control
  `ScopeVulnerability` when one of `Query.VulnerabilityIDs`, the id of the finding and its aliases,
  equals one of its ids, with no regard to case. A query with no ids never matches it, and
  `ScopeNamedVulnerability` is not a control. `Decide` then picks among the matching entries by id,
  so with a narrowed entry for CVE-A and a wide `ScopeVulnerability` entry on one package, a query
  for CVE-B gets the wide entry. `Decision.Vulnerability` is the waived item that matched, with its
  justification, so a tool can show it with the waived finding.
- `Normalize` drops a waived item with no id, and an item whose id equals an earlier id with no regard
  to case. The first item keeps its justification. The duplicate check of the server must fold case
  too, to agree with `match`.
- `ScopeLicense` waives the license findings of a scan gate: a denied license, a license outside
  the allow list and an unknown license.
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
- **Wrapped lists.** A message field with a field contract is a setting, never a group. Its path is
  the path of the field, and its value is the list of the one repeated field of its message. A set
  wrapper with no items is `[]any{}`, and an absent wrapper is not in `Values`. So
  `scan_gate.licenses.deny` with `{licenses: []}` denies nothing, and `scan_gate.licenses.allow` with
  `{licenses: []}` turns off the allow list of a lower layer. `scan_gate.licenses.unknown` is an
  `Enum` of `UnknownLicenseGate`. A message setting of another shape fails its capability as
  `ExceptionUnsupported`.
- **Package rule fields.** `TRUST_SCOPE_LICENSE` and `TRUST_SCOPE_NAMED_VULNERABILITY` map to
  `match.ScopeLicense` and `match.ScopeNamedVulnerability`. `PackageRule.vulnerabilities` becomes
  `Rule.Vulnerabilities`. A justification that the tool does not know becomes
  `match.JustificationNone`, so the tool shows the reason of the rule. The contract rules drop a
  trust entry whose ids and scope `NAMED_VULNERABILITY` do not go together.
- **Ordered enums.** An unknown value of an enum with the option `ordered` becomes the known value with
  the highest `strictness`. An unknown value of an enum with no order fails its capability.
- **Contract limits.** `Decode` runs protovalidate on each capability except `core`. A violation fails
  the capability: `Values` and `Sources` hold none of its settings, and `FailedCapabilities` names it.
  `Projection.Restore(previous)` takes a failed capability from the projection of the previous policy.
  An enum value 0 under `not_in: [0]` fails its capability the same way. The package rules of `core`
  have their own rules, one rule at a time, and never fail as a whole.
- `Fields` lists each contract field with a field contract, such as `binding_required` and
  `item_direction`, from the
  descriptor. A tool test checks its binding table against it.

The contract has no `FieldExceptionReason` for a value that fails its limits, so
`ExceptionInvalidValue` maps to `FIELD_EXCEPTION_REASON_UNSPECIFIED`.

## `policy/conformance`

pmg, the pull request check and vet each load a resolved policy through their own loader and pass
their own `match.Identity`. The corpus is the one check that their decisions agree for one policy. It
is the control for the risk "a bug in `match` lets a blocked package through".

### Run the corpus in a tool

A tool calls `conformance.Run` from its own test, with the subject kind of its policy and a function
that loads the policy and decides one query:

```go
func TestPolicyConformance(t *testing.T) {
	kind := specv1.SubjectKind_SUBJECT_KIND_ENDPOINT
	conformance.Run(t, kind, func(raw []byte, q conformance.Query) (conformance.Decision, error) {
		// The loader of the tool. conformance.Expect gives the tenant, the machine id and the
		// project id that the runner writes into the header.
		index, err := load(raw, conformance.Expect(kind))
		if err != nil {
			return conformance.Decision{}, err
		}

		// q.Package.Ecosystem is a name of pb.EcosystemName. pb.EcosystemFromName reads it.
		d := index.Decide(match.Query{
			Package:          pkg(q.Package),
			Control:          q.Control,
			Verdict:          q.Verdict,
			VulnerabilityIDs: q.VulnerabilityIDs,
			Now:              q.Now,
		})
		return conformance.Decision{Effect: d.Effect, RuleID: d.Rule.ID, VulnerabilityID: d.Vulnerability.ID}, nil
	}, match.ScopeMalware, match.ScopeCooldown, match.ScopeVulnerability, match.ScopeLicense)
}
```

- The last arguments of `Run` are the controls that the tool decides. The runner skips each check
  of another control. With no controls, every check runs. pmg decides `ScopeMalware` and
  `ScopeCooldown` only, so it names those two and skips the vulnerability and license checks.
- A tool that decides `ScopeVulnerability` must pass `Query.VulnerabilityIDs` to `match` and return
  `Decision.VulnerabilityID`. Until its loader does, the `named-vulnerability` case fails. This is
  intended: the case is the one shared check that a loader maps `PackageRule.vulnerabilities` and
  `TRUST_SCOPE_NAMED_VULNERABILITY`. It fails a loader that keeps the scope and drops the ids, so the
  entry waives nothing, and a loader that maps the entry to `ScopeVulnerability`, so it waives every
  vulnerability. The pull request check of control-tower decides the vulnerability control, so its
  run fails on this release of dry until it passes the ids.

- `raw` is the resolved policy on the wire. The runner writes a header for the subject kind: an
  endpoint gets the machine id `conformance.MachineID`, and a project gets the subject id
  `conformance.ProjectID`. The tenant is `conformance.Tenant`.
- `Query.Package` is the package as a tool sees it: the ecosystem name, the name as the registry or
  the manifest gives it, and the version, or "" when the tool has none. The tool puts it in canonical
  form, as it does for a real request.
- `Run` makes one subtest for each case and each check, as `TestX/exact-block/every-version`. A
  failure names the case file, the query, and the expected and the actual decision.
- The function gets a new copy of the bytes for each check. It returns an error only when the tool
  cannot load the policy or read the package. The runner reports the error as a failure.

dry runs the corpus through `project`, `match` and `pbidentity` in `TestCorpus`, for an endpoint and
for a project.

### Add a case

A case is two files in `policy/conformance/cases` with one base name, as `exact-block`:

- `exact-block.textproto` holds a `safedep.policy.spec.v1.ResolvedPolicy` in the text format, with
  only a `spec`. The runner writes the header, and `Cases` refuses a file that sets one. Write a time
  as `expires_at { seconds: 1796083200 }` with the date in a comment.
- `exact-block.json` holds the description and the checks:

```json
{
  "description": "What the case shows, in one or two sentences.",
  "now": "2026-11-01T12:00:00Z",
  "unknown_fields": [{"rule_id": "r_block_unknown", "field": 99}],
  "checks": [
    {
      "name": "every-version",
      "package": {"ecosystem": "npm", "name": "event-stream", "version": "3.3.6"},
      "control": "malware",
      "verdict": "none",
      "now": "2026-11-14T23:59:59Z",
      "want": {"effect": "block", "rule_id": "r_block_any"}
    }
  ]
}
```

- `control` is `malware`, `cooldown`, `vulnerability` or `license`: the controls of `match`.
- `verdict` is `none`, `malware` or `verified-malware`: the verdicts of `match`.
- `vulnerability_ids` are the id of the finding and its aliases. Only a check of the control
  `vulnerability` has them.
- `want.vulnerability_id` is the id of the waived vulnerability that the decision names, in the
  spelling of the rule. Only a `trust` decision of a check with `vulnerability_ids` has one.
- `want.effect` is `none`, `trust` or `block`. A `trust` or a `block` decision has the `rule_id` of
  the rule. A block entry with no id has the id `-`.
- `now` of the case is the time of each check. A check can set its own `now`.
- `unknown_fields` gives a package rule a varint field that the contract does not know. The text
  format cannot hold such a field. The field number must be one that the contract does not use.
- An unknown key in the JSON is an error, so a typo cannot skip a check.

Each case must pass against `project`, `match` and `pbidentity`. When a case fails because the code
and the spec disagree, do not change the case or the code alone. Decide the behaviour in the spec
first.

The directory is `cases`, not `testdata`, because a tool runs the corpus from another module, and the
go command can leave `testdata` out of a vendor directory.

| Case | Shows |
|---|---|
| `exact-block` | A block of one package, of one version, of an equal version, and with no rule id |
| `block-wins-over-trust` | A block entry wins over a trust entry with a lower id, for each control |
| `prefix-block` | Prefix boundaries for `/*` (npm, Go) and `.*` (Maven, PyPI, NuGet) |
| `vers-range` | A block and a trust `vers` range, inside, outside and at each bound |
| `trust-scopes` | A trust entry for each scope except `NAMED_VULNERABILITY`, under each control |
| `named-vulnerability` | A narrowed vulnerability entry waives its own id and an alias in any case, and no other id. A query with no ids gets none. Beside a wide entry, with the lower and with the higher rule id |
| `expired-rule` | A trust and a block entry before, at and after the expiry |
| `no-version` | A package with no version against rules with a version or a range |
| `unordered-versions` | GitHub Actions, with no version order: a range and one tag |
| `verified-malware` | No trust entry waives verified malware, also with every scope, under every control |
| `case-folding` | PyPI, NuGet and GitHub fold a name. PyPI also folds a version. npm does not fold |
| `unknown-field` | A block entry with an unknown field stays. A trust entry with one is dropped |
| `dropped-trust` | A trust entry with only an unknown scope, no expiry or a prefix selector is dropped |

## `policy/spdxlicense`

The package holds the one copy of the SPDX rules of the cloud policy. The pull request check and vet
call it, so a license gate decides the same way in each tool. The rules come from vet's
`internal/spdxlicense`. The package has no import of the API contract, so a vet package can wrap it.

```go
gate, err := spdxlicense.NewGate(spdxlicense.GateSettings{
	Deny:    deny,  // from scan_gate.licenses.deny
	Allow:   allow, // from scan_gate.licenses.allow
	Unknown: spdxlicense.UnknownAction(unknown), // the number of UnknownLicenseGate
})
if err != nil {
	return err
}

r := gate.Evaluate(pkg.Licenses) // Verdict: Denied, NotAllowed, Unknown or Pass. Fails: the gate fails.
```

- An SPDX id compares in any case. A deprecated id compares as its current id: `GPL-2.0` is
  `GPL-2.0-only`, and `GPL-2.0+` and `GPL-2.0-only+` are `GPL-2.0-or-later`. A `LicenseRef-` id
  compares exactly.
- An entry matches a license of the same form only: `GPL-2.0-only` does not match
  `GPL-2.0-only WITH Classpath-exception-2.0`.
- An or-later license fails a deny list when the list names it or each version that it allows. It
  passes an allow list when the list names it or one of those versions.
- A package passes when one choice of an OR expression fails no list. Every part of an AND must pass.
  The license values of one package join with AND. Each choice meets both lists at once, so
  `GPL-2.0-or-later OR MIT` fails allow `[GPL-3.0-only]` with deny `[GPL-2.0-or-later]`: the first
  choice is denied, and the second is not allowed.
- A denied or-later term stays denied. The allow list cannot pass it through one of its later
  versions.
- When no choice passes, the verdict is `Denied` if each choice has a denied term, and `NotAllowed`
  otherwise. A choice that fails both lists counts as denied. vet's `internal/spdxlicense` checked
  the two lists one after the other over the whole expression, and passed the example above. vet
  takes the new rule when it wraps this package.
- An empty deny list denies nothing, and an empty allow list is off. The unknown action decides only
  for a license that the lists do not fail.
- A license is unknown when the package has no license data, when a value is `NOASSERTION` or is not
  an SPDX expression, and when the value is `NONE` and no allow list is set. `NONE` fails an allow
  list.
- `ValidateEntry` checks one list entry of the policy contract. It refuses an expression, an id that
  is not on the SPDX License List, and the vet set names `osi-approved`, `fsf-libre` and
  `osi-approved-or-fsf-libre`. `NewGate` applies it to each entry. The server also refuses two entries
  that differ only in case. It is more lenient than the pattern of the contract: it takes white space,
  parentheses around a term and a lower case `with`. The server runs protovalidate first.
- `NewPolicy` and `Policy.Check` are the lower layer that the vet license control uses. They accept
  the set names.
- `list.json` holds the ids of the SPDX License List with their OSI and FSF flags, and the active
  license exception ids. The package reads only this embedded data. To update it, change the commit
  of `spdx/license-list-data` in `policy/spdxlicense/internal/gen`, run
  `go run ./policy/spdxlicense/internal/gen` from the root of the module, and read the diff. The
  command reads the network, so it has no `go:generate` directive and CI does not run it.
  `TestEmbeddedList` and `TestNoticeNamesTheListVersion` guard the data. The SPDX License List is under
  CC-BY-3.0. `NOTICE` gives its version, source and author, and a test fails when `NOTICE` names
  another version than `list.json`.

## Not in these packages

`policy/bind` (the binding table runner) comes in a later change. `cloud/configsync` fetches the
resolved policy and keeps the policy cache. See [cloud-configsync.md](cloud-configsync.md).
