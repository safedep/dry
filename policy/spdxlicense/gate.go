package spdxlicense

import (
	"errors"
	"fmt"
	"slices"
)

// UnknownAction is what a license gate does on a package with an unknown
// license. The values have the numbers of UnknownLicenseGate in the policy
// contract, so a reader converts the enum value of the policy with
// UnknownAction(n).
type UnknownAction int

const (
	// UnknownAllow passes the package and reports nothing.
	UnknownAllow UnknownAction = iota + 1

	// UnknownReport reports the package and does not fail the gate.
	UnknownReport

	// UnknownFail fails the gate.
	UnknownFail
)

var (
	// ErrInvalidEntry means that an entry of a deny or an allow list is not
	// an SPDX license id, an id WITH an exception or a LicenseRef.
	ErrInvalidEntry = errors.New("not an SPDX license id, an id WITH an exception or a LicenseRef")

	// ErrUnknownAction means that the action on an unknown license is not
	// UnknownAllow, UnknownReport or UnknownFail.
	ErrUnknownAction = errors.New("unknown action on an unknown license")
)

// GateSettings are the settings of a license gate, as the policy contract
// gives them in scan_gate.licenses.
type GateSettings struct {
	// Deny holds the licenses that fail the gate. An empty list denies no
	// license.
	Deny []string

	// Allow holds the licenses that a package can have. An empty list turns
	// the allow list off.
	Allow []string

	// Unknown decides only for a package that the lists do not fail.
	Unknown UnknownAction
}

// Gate applies a license gate of a policy to the license values of one
// package.
type Gate struct {
	policy  *Policy
	unknown UnknownAction
}

// NewGate checks each entry with ValidateEntry and the unknown action. An
// invalid entry is an error, so a typo cannot open the gate.
func NewGate(s GateSettings) (*Gate, error) {
	if !slices.Contains([]UnknownAction{UnknownAllow, UnknownReport, UnknownFail}, s.Unknown) {
		return nil, fmt.Errorf("%w: %d", ErrUnknownAction, s.Unknown)
	}

	if err := validateList(s.Deny); err != nil {
		return nil, fmt.Errorf("deny: %w", err)
	}

	if err := validateList(s.Allow); err != nil {
		return nil, fmt.Errorf("allow: %w", err)
	}

	p, err := NewPolicy(s.Allow, s.Deny)
	if err != nil {
		return nil, err
	}

	return &Gate{policy: p, unknown: s.Unknown}, nil
}

// Evaluate checks the license values of one package, as a package registry
// or an SBOM gives them. The verdict is the finding kind: Denied,
// NotAllowed, Unknown or Pass. The gate applies the deny list, then the
// allow list, and the unknown action decides only for a license that the
// lists do not fail. Under UnknownAllow an unknown license passes, and
// under UnknownFail it fails the gate.
func (g *Gate) Evaluate(values []string) Result {
	r := g.policy.Check(Parse(values))
	if r.Verdict != Unknown {
		return r
	}

	switch g.unknown {
	case UnknownReport:
		return r
	case UnknownFail:
		r.Fails = true
		return r
	default:
		return Result{Verdict: Pass}
	}
}

// ValidateEntry checks one entry of a deny or an allow list of the policy
// contract: an id of the SPDX License List with an optional +, a deprecated
// id included, an "id WITH exception" term, or a LicenseRef. It refuses a
// set name, as osi-approved, because only a vet config can hold one. It
// does not compare two entries: the server refuses two entries that differ
// only in case.
//
// It is more lenient than the pattern of the contract: it takes white space
// around the entry, parentheses around a term and a lower case "with". The
// server runs protovalidate first, so a stored entry matches the pattern
// too.
func ValidateEntry(entry string) error {
	if slices.Contains(Sets, entry) {
		return fmt.Errorf("%q: %w: a policy list cannot hold a set name", entry, ErrInvalidEntry)
	}

	n, err := parseExpression(entry)
	if err != nil {
		return fmt.Errorf("%q: %w: %w", entry, ErrInvalidEntry, err)
	}

	if n.op != opTerm {
		return fmt.Errorf("%q: %w: an entry is one license, not an expression", entry, ErrInvalidEntry)
	}

	return nil
}

func validateList(entries []string) error {
	var errs []error
	for _, e := range entries {
		errs = append(errs, ValidateEntry(e))
	}

	return errors.Join(errs...)
}
