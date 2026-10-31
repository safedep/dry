package match

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Effect is what a package rule does.
type Effect int

const (
	// EffectNone means that no rule decided. A rule with this effect, or with
	// an effect that the package does not know, is dropped.
	EffectNone Effect = iota

	// EffectTrust waives the verdicts of the scopes that the rule names. It
	// never waives a verdict of verified malware.
	EffectTrust

	// EffectBlock blocks the packages that the rule selects. It wins over
	// every trust entry.
	EffectBlock
)

func (e Effect) String() string {
	switch e {
	case EffectNone:
		return "none"
	case EffectTrust:
		return "trust"
	case EffectBlock:
		return "block"
	default:
		return fmt.Sprintf("effect(%d)", int(e))
	}
}

// Scope names a control, and the verdicts of that control that a trust entry
// waives.
type Scope int

const (
	// ScopeMalware is a malware verdict that is not verified, such as a
	// suspicious one.
	ScopeMalware Scope = iota + 1

	// ScopeCooldown is the dependency cooldown.
	ScopeCooldown

	// ScopeVulnerability is the vulnerability findings of a scan gate.
	ScopeVulnerability
)

func (s Scope) known() bool {
	return s >= ScopeMalware && s <= ScopeVulnerability
}

func (s Scope) String() string {
	switch s {
	case ScopeMalware:
		return "malware"
	case ScopeCooldown:
		return "cooldown"
	case ScopeVulnerability:
		return "vulnerability"
	default:
		return fmt.Sprintf("scope(%d)", int(s))
	}
}

// OriginKind says where a rule comes from.
type OriginKind int

const (
	// OriginCloud is a rule of the resolved policy of the tenant.
	OriginCloud OriginKind = iota + 1

	// OriginLocal is a rule of a local config file of the tool.
	OriginLocal
)

// Origin says where a rule comes from. A tool uses it in a block message and
// in a decision citation.
type Origin struct {
	Kind OriginKind

	// Path is the config file of a local rule.
	Path string
}

// Rule is a trust entry or a block entry.
type Rule struct {
	// ID is the rule id, as in "r_9c1x4kq7dm".
	ID string

	Effect Effect

	// PURL selects one package, one version of a package, or every package
	// under a prefix that ends with "/*" or ".*". Only a block entry can use a
	// prefix.
	PURL string

	// Vers is an optional version range, as in "vers:npm/>=1.2.0|<2.0.0".
	Vers string

	// Scopes are the scopes that a trust entry waives. A block entry has none.
	Scopes []Scope

	// Reason says why the rule exists.
	Reason string

	// ExpiresAt is the time at which the rule stops to apply. The zero value
	// means that the rule does not expire.
	ExpiresAt time.Time

	Origin Origin
}

// BlockRuleIDNone is the id of a block entry that has no id. A block entry
// with no id still applies, because a block that a tool drops is a block
// that does not apply.
const BlockRuleIDNone = "-"

var (
	// ErrUnknownEffect means that the rule has an effect that the package
	// does not know. A later effect never blocks, so the drop never blocks
	// less.
	ErrUnknownEffect = errors.New("unknown effect")

	// ErrNoRuleID means that a trust entry has no rule id.
	ErrNoRuleID = errors.New("trust entry has no rule id")

	// ErrNoExpiry means that a trust entry has no expiry. The contract
	// requires one (package_rule.trust_needs_expiry), and a cache that a user
	// edits has no server check, so a trust entry with none would never end.
	ErrNoExpiry = errors.New("trust entry has no expiry")

	// ErrNoKnownScope means that a trust entry has no scope that the package
	// knows. An empty scope list never means "all".
	ErrNoKnownScope = errors.New("trust entry has no known scope")

	// ErrTrustPrefix means that a trust entry selects a prefix. A prefix
	// would also trust packages that no one reviewed.
	ErrTrustPrefix = errors.New("trust entry selects a prefix")

	// ErrInvalidVers means that the vers range of a rule does not parse,
	// names another ecosystem than the PURL, or has bounds out of version
	// order.
	ErrInvalidVers = errors.New("invalid vers range")

	// ErrInvalidSelector means that the identity cannot parse the PURL of a
	// rule.
	ErrInvalidSelector = errors.New("invalid selector")
)

// Normalize applies the rules that need no package identity. It returns a
// copy of the rule:
//   - with the scopes that the package knows, in order and once each, for a
//     trust entry, and no scope for a block entry;
//   - with the id BlockRuleIDNone for a block entry with no id.
//
// It returns an error for a rule that a tool must drop: an unknown effect, or
// a trust entry with no id, no expiry, no known scope, a prefix selector or a
// vers range that does not parse. A block entry is never dropped here. The
// checks that need the identity, such as the ecosystem of the vers range,
// run in NewIndex and Validate. NewIndex calls
// Normalize, so a caller needs it only to report dropped rules early.
func Normalize(r Rule) (Rule, error) {
	switch r.Effect {
	case EffectBlock:
		r.Scopes = nil
		if r.ID == "" {
			r.ID = BlockRuleIDNone
		}

		return r, nil

	case EffectTrust:
		return normalizeTrust(r)

	default:
		return r, fmt.Errorf("%w: %d", ErrUnknownEffect, int(r.Effect))
	}
}

func normalizeTrust(r Rule) (Rule, error) {
	if r.ID == "" {
		return r, ErrNoRuleID
	}

	if r.ExpiresAt.IsZero() {
		return r, ErrNoExpiry
	}

	scopes := make([]Scope, 0, len(r.Scopes))
	for _, s := range r.Scopes {
		if s.known() && !slices.Contains(scopes, s) {
			scopes = append(scopes, s)
		}
	}

	if len(scopes) == 0 {
		return r, ErrNoKnownScope
	}

	slices.Sort(scopes)
	r.Scopes = scopes

	if isPrefix(r.PURL) {
		return r, ErrTrustPrefix
	}

	if r.Vers != "" {
		if _, err := ParseVers(r.Vers); err != nil {
			return r, fmt.Errorf("%w: %w", ErrInvalidVers, err)
		}
	}

	return r, nil
}

// isPrefix reports whether a selector is a prefix. An encoded "*" counts, so
// it cannot pass as one package.
func isPrefix(purl string) bool {
	return strings.Contains(purl, "*") || strings.Contains(strings.ToLower(purl), "%2a")
}

// purlType returns the type of a PURL in lower case, or "" when the PURL has
// none.
func purlType(purl string) string {
	scheme, rest, ok := strings.Cut(purl, ":")
	if !ok || !strings.EqualFold(scheme, "pkg") {
		return ""
	}

	typ, _, ok := strings.Cut(strings.TrimLeft(rest, "/"), "/")
	if !ok {
		return ""
	}

	return strings.ToLower(typ)
}
