// Package conformance holds the shared corpus of package rule decisions, and
// the runner that checks the decisions of a tool against it.
//
// pmg, the pull request check and vet each load a resolved policy through
// their own loader and pass their own match.Identity. This corpus is the one
// check that their decisions agree for one policy. It is the control for the
// risk "a bug in match lets a blocked package through".
//
// A tool runs the corpus from its own test with one call:
//
//	func TestPolicyConformance(t *testing.T) {
//		conformance.Run(t, specv1.SubjectKind_SUBJECT_KIND_ENDPOINT,
//			func(policy []byte, q conformance.Query) (conformance.Decision, error) {
//				// Load the policy bytes through the loader of the tool, with
//				// conformance.Expect for the header check, and decide q.
//			})
//	}
//
// The runner gives the policy as the bytes of a resolved policy on the wire,
// with a header for the subject kind of the tool. So the test runs the same
// loader as the tool does on a fetched policy.
//
// The case files are in the directory "cases" of this package, and the
// package embeds them. docs/policy.md says how to add a case.
package conformance
