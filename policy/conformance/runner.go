package conformance

import (
	"slices"
	"testing"

	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
)

// DecideFunc loads a resolved policy through the loader of a tool and decides
// one query. policy is the bytes of the policy on the wire. The function gets
// a new copy of the bytes for each check.
//
// It returns an error only when the tool cannot load the policy or cannot
// read the package. The runner reports the error as a failure.
type DecideFunc func(policy []byte, q Query) (Decision, error)

// Run runs every check of the corpus through decide, in one subtest for each
// case and each check. kind is the subject kind that the tool loads a policy
// for: the runner writes the header for it, and Expect gives the values that
// the loader must expect.
func Run(t *testing.T, kind specv1.SubjectKind, decide DecideFunc) {
	t.Helper()

	cases, err := Cases()
	if err != nil {
		t.Fatalf("cannot load the conformance corpus: %v", err)
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			raw, err := c.Policy(kind)
			if err != nil {
				t.Fatalf("%s: cannot build the policy: %v", c.File, err)
			}

			for _, chk := range c.Checks {
				t.Run(chk.Name, func(t *testing.T) {
					verify(t, c, chk, raw, decide)
				})
			}
		})
	}
}

// errorer is the part of testing.TB that verify uses, so a test can record
// the failure message.
type errorer interface {
	Helper()
	Errorf(format string, args ...any)
}

// verify runs one check. The message names the case file and the check, and
// the expected and the actual decision, so a person finds the case with no
// other output.
func verify(t errorer, c Case, chk Check, raw []byte, decide DecideFunc) bool {
	t.Helper()

	// A tool that changes the bytes must not change the policy of the next
	// check.
	got, err := decide(slices.Clone(raw), chk.Query)
	if err != nil {
		t.Errorf("%s, check %q: the tool returned an error: %v", c.File, chk.Name, err)
		return false
	}

	if got != chk.Want {
		t.Errorf("%s, check %q: %s: want %s, got %s", c.File, chk.Name, chk.Query, chk.Want, got)
		return false
	}

	return true
}
