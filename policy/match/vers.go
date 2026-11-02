package match

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// comparator is the comparator of one vers constraint.
type comparator int

const (
	compEqual comparator = iota
	compNotEqual
	compLess
	compLessEqual
	compGreater
	compGreaterEqual
)

// comparatorTokens lists the two-character tokens first, so "<=" never parses
// as "<" and a version that starts with "=".
var comparatorTokens = []struct {
	token string
	comp  comparator
}{
	{"!=", compNotEqual},
	{"<=", compLessEqual},
	{">=", compGreaterEqual},
	{"<", compLess},
	{">", compGreater},
	{"=", compEqual},
}

func (c comparator) String() string {
	for _, t := range comparatorTokens {
		if t.comp == c {
			return t.token
		}
	}

	return "?"
}

func (c comparator) lower() bool {
	return c == compLess || c == compLessEqual
}

func (c comparator) greater() bool {
	return c == compGreater || c == compGreaterEqual
}

// satisfiedBy reports whether a version that compares to the constraint
// version as cmp satisfies the constraint.
func (c comparator) satisfiedBy(cmp int) bool {
	switch c {
	case compEqual:
		return cmp == 0
	case compNotEqual:
		return cmp != 0
	case compLess:
		return cmp < 0
	case compLessEqual:
		return cmp <= 0
	case compGreater:
		return cmp > 0
	case compGreaterEqual:
		return cmp >= 0
	default:
		return false
	}
}

type constraint struct {
	comp    comparator
	version string
}

// Range is a version range in the vers form of the package-url project, as
// in "vers:npm/>=1.2.0|<2.0.0".
type Range struct {
	// Scheme is the versioning scheme in lower case. It must equal the type
	// of the PURL that the range applies to.
	Scheme string

	all bool

	// points holds the "=" and "!=" constraints.
	points []constraint

	// bounds holds the "<", "<=", ">" and ">=" constraints in the written
	// order, which the vers specification sorts by version.
	bounds []constraint
}

const versScheme = "vers:"

// ParseVers parses a vers range. It applies the rules of the vers
// specification that need no version order:
//   - "|" separates constraints, and white space is not significant;
//   - a constraint without a comparator means "=";
//   - "*" alone selects every version;
//   - each version is unique;
//   - the "<", "<=", ">" and ">=" constraints alternate between a lower and a
//     greater comparator.
//
// ParseVers refuses a range of only "!=" constraints. The vers specification
// gives it no version, and the plain reading gives it every other version.
// Each reading fails open for one effect: a block that matches nothing, or a
// trust of every other version. So the range is invalid, and the fail-closed
// rules apply: a block entry matches every version, and a trust entry is
// dropped.
//
// The order of the bounds needs the order of the ecosystem. NewIndex and
// Validate check it through the Identity, and Contains checks it again for
// each version that it tests. ParseVers accepts "=" and "!=" constraints at
// any position, because each one checks one version.
func ParseVers(s string) (Range, error) {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)

	if !strings.HasPrefix(s, versScheme) {
		return Range{}, fmt.Errorf("vers %q does not start with %q", s, versScheme)
	}

	scheme, list, ok := strings.Cut(s[len(versScheme):], "/")
	if !ok {
		return Range{}, fmt.Errorf("vers %q has no versioning scheme", s)
	}

	scheme = strings.ToLower(scheme)
	if !validScheme(scheme) {
		return Range{}, fmt.Errorf("vers %q has an invalid versioning scheme", s)
	}

	r := Range{Scheme: scheme}
	if list == "*" {
		r.all = true
		return r, nil
	}

	// The order check compares pairs of constraints, so a range with no
	// limit would make Validate and NewIndex slow. The contract caps vers at
	// 512 characters, which holds fewer constraints than this limit.
	if n := strings.Count(list, "|") + 1; n > maxVersConstraints {
		return Range{}, fmt.Errorf("vers %q has %d constraints, more than %d", s, n, maxVersConstraints)
	}

	seen := map[string]bool{}
	for part := range strings.SplitSeq(list, "|") {
		c, err := parseConstraint(part)
		if err != nil {
			return Range{}, fmt.Errorf("vers %q: %w", s, err)
		}

		if seen[c.version] {
			return Range{}, fmt.Errorf("vers %q has the version %q twice", s, c.version)
		}
		seen[c.version] = true

		if c.comp == compEqual || c.comp == compNotEqual {
			r.points = append(r.points, c)
		} else {
			r.bounds = append(r.bounds, c)
		}
	}

	if len(r.bounds) == 0 && allNotEqual(r.points) {
		return Range{}, fmt.Errorf("vers %q has only \"!=\" constraints", s)
	}

	for i := 1; i < len(r.bounds); i++ {
		if r.bounds[i-1].comp.lower() == r.bounds[i].comp.lower() {
			return Range{}, fmt.Errorf("vers %q: %q follows %q, but the comparators must alternate",
				s, r.bounds[i].comp, r.bounds[i-1].comp)
		}
	}

	return r, nil
}

func validScheme(scheme string) bool {
	if scheme == "" {
		return false
	}

	for _, c := range scheme {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '.' && c != '+' && c != '-' {
			return false
		}
	}

	return true
}

func parseConstraint(part string) (constraint, error) {
	if part == "" {
		return constraint{}, errors.New("empty constraint")
	}

	if part == "*" {
		return constraint{}, errors.New(`"*" must be the only constraint`)
	}

	c := constraint{comp: compEqual}
	raw := part
	for _, t := range comparatorTokens {
		if rest, ok := strings.CutPrefix(part, t.token); ok {
			c.comp = t.comp
			raw = rest
			break
		}
	}

	if raw == "" {
		return constraint{}, fmt.Errorf("constraint %q has no version", part)
	}

	if strings.ContainsAny(raw[:1], "<>=!*") {
		return constraint{}, fmt.Errorf("constraint %q has an invalid comparator", part)
	}

	version, err := url.PathUnescape(raw)
	if err != nil {
		return constraint{}, fmt.Errorf("constraint %q: %w", part, err)
	}

	c.version = version
	return c, nil
}

// String returns the range in the vers form. Each version is
// percent-encoded, so the result parses back to the same range.
func (r Range) String() string {
	var b strings.Builder
	b.WriteString(versScheme)
	b.WriteString(r.Scheme)
	b.WriteString("/")

	if r.all {
		b.WriteString("*")
		return b.String()
	}

	for i, c := range append(append([]constraint{}, r.points...), r.bounds...) {
		if i > 0 {
			b.WriteString("|")
		}

		b.WriteString(c.comp.String())
		b.WriteString(escapeVersion(c.version))
	}

	return b.String()
}

// escapeVersion encodes a version for a vers constraint. url.PathEscape keeps
// "=", so a version that starts with "=" would read back as part of the
// comparator: "<" and "=1" would print as "<=1".
func escapeVersion(version string) string {
	return strings.ReplaceAll(url.PathEscape(version), "=", "%3D")
}

// maxVersConstraints limits the constraints of one vers range.
const maxVersConstraints = 256

// errConflict means that a "=" and a "!=" constraint name one version.
var errConflict = errors.New("vers has a \"=\" and a \"!=\" constraint of one version")

// errUnordered means that the bound constraints are not in the order of the
// versions, as the vers specification requires.
var errUnordered = errors.New("vers constraints are not in version order")

// Contains reports whether the version of the package is in the range. It
// follows the check of the vers specification:
//  1. "*" contains every version.
//  2. A version equal to the version of a "=" constraint is in the range. A
//     version equal to the version of a "!=" constraint is not.
//  3. The "<", "<=", ">" and ">=" constraints decide, as a list of
//     intervals. With no such constraint, the version is not in the range.
//
// It returns an error when the package has no version, when the package
// cannot compare a version, or when the bound constraints are not in version
// order for this version. The caller decides what an error means: a block
// entry matches, and a trust entry does not.
func (r Range) Contains(p Package) (bool, error) {
	// A package with no version fails before "*" too. Otherwise "vers:npm/*"
	// would let a trust entry match a package with no version, which every
	// other range refuses.
	if p.Version() == "" {
		return false, errors.New("package has no version")
	}

	if r.all {
		return true, nil
	}

	// Two points can name one version in two texts, as "=1.0.0|!=1.0.0+build"
	// does under npm. When they disagree, the written order must not decide,
	// so the check fails: a block entry matches and a trust entry does not.
	var matched, in bool
	for _, c := range r.points {
		cmp, err := p.CompareVersion(c.version)
		if err != nil {
			return false, err
		}

		if cmp != 0 {
			continue
		}

		eq := c.comp == compEqual
		if matched && eq != in {
			return false, errConflict
		}
		matched, in = true, eq
	}

	if matched {
		return in, nil
	}

	if len(r.bounds) == 0 {
		return false, nil
	}

	cmps := make([]int, len(r.bounds))
	for i, c := range r.bounds {
		cmp, err := p.CompareVersion(c.version)
		if err != nil {
			return false, err
		}
		cmp = sign(cmp)

		// The bounds go up in version, so the order of the tested version
		// against them can only go down. A rise means that the written
		// order is not the version order, and the intervals mean nothing.
		if i > 0 && cmp > cmps[i-1] {
			return false, errUnordered
		}
		cmps[i] = cmp
	}

	return r.boundsContain(cmps), nil
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

func allNotEqual(points []constraint) bool {
	for _, c := range points {
		if c.comp != compNotEqual {
			return false
		}
	}

	return true
}

// boundsContain applies the pairwise check of the vers specification to the
// order of the tested version against each bound.
func (r Range) boundsContain(cmps []int) bool {
	b := r.bounds
	if len(b) == 1 {
		return b[0].comp.satisfiedBy(cmps[0])
	}

	if b[0].comp.lower() && b[0].comp.satisfiedBy(cmps[0]) {
		return true
	}

	last := len(b) - 1
	if b[last].comp.greater() && b[last].comp.satisfiedBy(cmps[last]) {
		return true
	}

	for i := 0; i < last; i++ {
		cur, next := b[i], b[i+1]
		if cur.comp.greater() && next.comp.lower() &&
			cur.comp.satisfiedBy(cmps[i]) && next.comp.satisfiedBy(cmps[i+1]) {
			return true
		}
	}

	return false
}

// checkOrder checks that every version of the range can be compared, that no
// "=" and "!=" constraint name one version, and that the bounds go up in
// version, under the order of the ecosystem of
// the selector. It builds a package of the selector at each version through
// the identity. The order depends on the ecosystem only, so a prefix selector
// gets the name "x" under its prefix. A version that cannot be compared, also
// in a range with one constraint, would fail at decision time and make a
// block match every version, so the range is invalid here.
func (r Range) checkOrder(identity Identity, selector string) error {
	for _, c := range append(append([]constraint{}, r.points...), r.bounds...) {
		pkg, err := identity.Package(purlAt(selector, c.version))
		if err != nil {
			return fmt.Errorf("version %q: %w", c.version, err)
		}

		if _, err := pkg.CompareVersion(c.version); err != nil {
			return fmt.Errorf("%w: version %q: %w", errUnordered, c.version, err)
		}
	}

	for i, a := range r.points {
		pkg, err := identity.Package(purlAt(selector, a.version))
		if err != nil {
			return fmt.Errorf("version %q: %w", a.version, err)
		}

		for _, b := range r.points[i+1:] {
			if a.comp == b.comp {
				continue
			}

			cmp, err := pkg.CompareVersion(b.version)
			if err != nil {
				return fmt.Errorf("version %q: %w", b.version, err)
			}

			if cmp == 0 {
				return fmt.Errorf("%w: %q and %q", errConflict, a.version, b.version)
			}
		}
	}

	for i := 0; i+1 < len(r.bounds); i++ {
		pkg, err := identity.Package(purlAt(selector, r.bounds[i].version))
		if err != nil {
			return fmt.Errorf("bound %q: %w", r.bounds[i].version, err)
		}

		cmp, err := pkg.CompareVersion(r.bounds[i+1].version)
		if err != nil {
			return fmt.Errorf("bound %q: %w", r.bounds[i+1].version, err)
		}

		if cmp >= 0 {
			return fmt.Errorf("%w: %q does not come before %q", errUnordered, r.bounds[i].version, r.bounds[i+1].version)
		}
	}

	return nil
}

// purlAt returns the selector with a version. A prefix selector gets the
// name "x" in place of its "*".
func purlAt(selector, version string) string {
	base, suffix := selector, ""
	if i := strings.IndexAny(selector, "?#"); i >= 0 {
		base, suffix = selector[:i], selector[i:]
	}

	if stem, ok := strings.CutSuffix(base, "*"); ok {
		base = stem + "x"
	}

	// url.PathEscape keeps "@", but the PURL parser takes the last "@" as the
	// start of the version. A bound "2@0" would turn into the version "0", and
	// the order check would test another version than the range holds.
	return base + "@" + strings.ReplaceAll(url.PathEscape(version), "@", "%40") + suffix
}
