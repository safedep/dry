package match

import (
	"cmp"
	"fmt"
	"slices"
)

// Dropped is a rule that is not in the index, and the reason.
type Dropped struct {
	Rule Rule
	Err  error
}

// entry is one rule in the index, with its selector in parsed form.
type entry struct {
	rule Rule

	// version is the canonical version of the selector PURL, or "".
	version string

	// rng is the vers range of the rule, or nil.
	rng *Range
}

// Index finds the package rules of a package. It holds a map from the
// NameKey of each exact selector to its rules, and a map from each prefix key
// to its rules. A check reads one entry of the first map, and one entry of
// the second map for each distinct prefix length. So the cost of a check does
// not grow with the number of rules.
//
// An Index is safe for concurrent use. To change the rules, build a new
// Index.
type Index struct {
	exact    map[string][]*entry
	prefixes map[string][]*entry

	// prefixLengths holds each distinct length of a prefix key, in order.
	prefixLengths []int

	rules []Rule
}

// NewIndex builds the index of the rules. It drops a rule that Normalize
// drops, a rule whose selector the identity cannot parse, and a trust entry
// whose vers range is invalid for its selector (see Validate). A block entry
// with an invalid vers range stays in the index and matches every version of
// its packages. It returns each dropped rule with the reason.
func NewIndex(rules []Rule, identity Identity) (*Index, []Dropped) {
	ix := &Index{
		exact:    map[string][]*entry{},
		prefixes: map[string][]*entry{},
	}

	var dropped []Dropped
	for _, raw := range rules {
		r, err := Normalize(raw)
		if err != nil {
			dropped = append(dropped, Dropped{Rule: raw, Err: err})
			continue
		}

		c, err := compile(r, identity)
		if err == nil && c.rangeErr != nil && r.Effect == EffectTrust {
			err = c.rangeErr
		}

		if err != nil {
			dropped = append(dropped, Dropped{Rule: r, Err: err})
			continue
		}

		if c.prefix {
			ix.prefixes[c.key] = append(ix.prefixes[c.key], c.entry)
		} else {
			ix.exact[c.key] = append(ix.exact[c.key], c.entry)
		}

		ix.rules = append(ix.rules, r)
	}

	for _, list := range ix.exact {
		slices.SortFunc(list, compareEntries)
	}

	for key, list := range ix.prefixes {
		slices.SortFunc(list, compareEntries)
		ix.prefixLengths = append(ix.prefixLengths, len(key))
	}

	slices.Sort(ix.prefixLengths)
	ix.prefixLengths = slices.Compact(ix.prefixLengths)
	slices.SortFunc(ix.rules, compareRules)

	return ix, dropped
}

// Validate applies every check of NewIndex to one rule, and also refuses a
// block entry with an invalid vers range, which NewIndex keeps. A server
// calls it before it stores a rule, so a stored rule never drops out of the
// index of a tool and never matches more than its admin wrote.
func Validate(r Rule, identity Identity) error {
	normalized, err := Normalize(r)
	if err != nil {
		return err
	}

	c, err := compile(normalized, identity)
	if err != nil {
		return err
	}

	return c.rangeErr
}

// compiled is a rule with its selector and range in parsed form.
type compiled struct {
	entry  *entry
	key    string
	prefix bool

	// rangeErr is the reason that the vers range does not apply. A block
	// entry survives it and matches every version. A trust entry does not.
	rangeErr error
}

// compile parses the selector and the vers range of a normalized rule. It
// returns an error when the identity cannot parse the selector.
func compile(r Rule, identity Identity) (compiled, error) {
	if identity == nil {
		return compiled{}, fmt.Errorf("%w: no identity", ErrInvalidSelector)
	}

	c := compiled{entry: &entry{rule: r}, prefix: isPrefix(r.PURL)}
	if c.prefix {
		key, err := identity.Prefix(r.PURL)
		if err != nil {
			return compiled{}, fmt.Errorf("%w: %w", ErrInvalidSelector, err)
		}
		c.key = key
	} else {
		pkg, err := identity.Package(r.PURL)
		if err != nil {
			return compiled{}, fmt.Errorf("%w: %w", ErrInvalidSelector, err)
		}
		c.key = pkg.NameKey()
		c.entry.version = pkg.Version()
	}

	if r.Vers == "" {
		return c, nil
	}

	rng, err := compileRange(r, identity, c.entry.version)
	if err != nil {
		// The block entry matches every version: neither the version of the
		// PURL nor the range narrows it.
		c.rangeErr = err
		c.entry.version = ""
		return c, nil
	}

	c.entry.rng = &rng
	return c, nil
}

func compileRange(r Rule, identity Identity, version string) (Range, error) {
	if version != "" {
		return Range{}, fmt.Errorf("%w: the purl has a version and the rule has a vers range", ErrInvalidVers)
	}

	rng, err := ParseVers(r.Vers)
	if err != nil {
		return Range{}, fmt.Errorf("%w: %w", ErrInvalidVers, err)
	}

	typ := purlType(r.PURL)
	if !sameEcosystem(identity, rng.Scheme, typ) {
		return Range{}, fmt.Errorf("%w: vers type %q is not the ecosystem of the purl type %q", ErrInvalidVers, rng.Scheme, typ)
	}

	if err := rng.checkBoundOrder(identity, r.PURL); err != nil {
		return Range{}, fmt.Errorf("%w: %w", ErrInvalidVers, err)
	}

	return rng, nil
}

// sameEcosystem reports whether two PURL types name one ecosystem, such as
// "pypi" and "pip". The aliases belong to the identity, so it asks the
// identity for one name under each type: the NameKeys are equal only when
// the ecosystem and the fold are the same.
func sameEcosystem(identity Identity, a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	if a == b {
		return true
	}

	pa, err := identity.Package("pkg:" + a + "/x")
	if err != nil {
		return false
	}

	pb, err := identity.Package("pkg:" + b + "/x")
	if err != nil {
		return false
	}

	return pa.NameKey() == pb.NameKey()
}

// Rules returns the rules in the index, in normalized form and in the order
// of their ids. A tool counts or lists them with it.
func (ix *Index) Rules() []Rule {
	return slices.Clone(ix.rules)
}

func compareEntries(a, b *entry) int {
	return compareRules(a.rule, b.rule)
}

// compareRules orders rules by id, so the same rule always decides. The
// other fields break a tie, such as a local and a cloud rule of one id, so
// the order never depends on the order of the input.
func compareRules(a, b Rule) int {
	return cmp.Or(
		cmp.Compare(a.ID, b.ID),
		cmp.Compare(a.Effect, b.Effect),
		cmp.Compare(a.PURL, b.PURL),
		cmp.Compare(a.Vers, b.Vers),
		slices.Compare(a.Scopes, b.Scopes),
		a.ExpiresAt.Compare(b.ExpiresAt),
		cmp.Compare(a.Reason, b.Reason),
		cmp.Compare(a.Origin.Kind, b.Origin.Kind),
		cmp.Compare(a.Origin.Path, b.Origin.Path),
	)
}
