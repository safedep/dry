// Package match decides which package rule applies to a package: a block
// entry or a trust entry of a policy.
//
// The package holds the client invariants of the cloud policy, so that every
// tool and the pull request check apply package rules the same way:
//
//   - A block entry wins over a trust entry.
//   - A trust entry waives a verdict only for a control that its scopes name.
//   - No trust entry waives a verdict of verified malware, whatever its scopes
//     are. This is the verified malware guard.
//   - A trust entry with a prefix selector, no known scope, no rule id, no
//     expiry or an invalid vers range is dropped. A block entry with an
//     invalid vers range matches every version. Validate refuses both, so a
//     server never stores them.
//   - A rule applies while now is before its expiry. The caller passes now,
//     so a long-running tool drops an expired entry with no reload.
//
// The package has no import of the SafeDep API contract. Its types are plain
// Go, and it compares packages through the Identity interface that the tool
// supplies. So a tool whose rules forbid an import of dry/api/pb can use it.
// Package policy/pbidentity gives an Identity on the rules of dry/api/pb.
//
// The package holds no tool precedence beyond the order of Index.Decide. The
// tool places Decide in its own flow.
package match
