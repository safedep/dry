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
// whose PURL has a version and that also has a vers range. A block entry
// whose vers range does not parse, or that has both a version and a vers
// range, stays in the index and matches every version of its packages. It
// returns each dropped rule with the reason.
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

		if err := ix.add(r, identity); err != nil {
			dropped = append(dropped, Dropped{Rule: r, Err: err})
			continue
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

func (ix *Index) add(r Rule, identity Identity) error {
	if identity == nil {
		return fmt.Errorf("%w: no identity", ErrInvalidSelector)
	}

	e := &entry{rule: r}
	if r.Vers != "" {
		if rng, err := parseRuleVers(r); err == nil {
			e.rng = &rng
		}
	}

	if isPrefix(r.PURL) {
		key, err := identity.Namespace(r.PURL)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSelector, err)
		}

		ix.prefixes[key] = append(ix.prefixes[key], e)
		return nil
	}

	pkg, err := identity.Package(r.PURL)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSelector, err)
	}

	e.version = pkg.Version()
	if e.version != "" && r.Vers != "" {
		if r.Effect == EffectTrust {
			return fmt.Errorf("%w: the purl has a version and the rule has a vers range", ErrInvalidVers)
		}

		e.version, e.rng = "", nil
	}

	key := pkg.NameKey()
	ix.exact[key] = append(ix.exact[key], e)

	return nil
}

// Rules returns the rules in the index, in normalized form and in the order
// of their ids. A tool counts or lists them with it.
func (ix *Index) Rules() []Rule {
	return slices.Clone(ix.rules)
}

func compareEntries(a, b *entry) int {
	return compareRules(a.rule, b.rule)
}

// compareRules orders rules by id, so the same rule always decides. The PURL
// and the range break a tie between a local and a cloud rule of one id.
func compareRules(a, b Rule) int {
	return cmp.Or(
		cmp.Compare(a.ID, b.ID),
		cmp.Compare(a.PURL, b.PURL),
		cmp.Compare(a.Vers, b.Vers),
		cmp.Compare(a.Origin.Kind, b.Origin.Kind),
	)
}
