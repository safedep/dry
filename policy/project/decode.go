package project

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	recordv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/record/v1"
	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
	"google.golang.org/protobuf/proto"

	"github.com/safedep/dry/policy/match"
)

var (
	// ErrDecode means that the bytes are not a resolved policy.
	ErrDecode = errors.New("cannot decode the resolved policy")

	// ErrHeader means that the header, the tenant or the subject failed its
	// check. The tool rejects the whole document.
	ErrHeader = errors.New("resolved policy header check failed")
)

// Expect holds what the header of a resolved policy must say.
type Expect struct {
	// Tenant is the tenant domain of the login. When it is empty, Decode
	// skips only the tenant comparison: a tool with no login applies the
	// cache that it fetched for its own tenant before.
	Tenant string

	// SubjectKind is the kind of the subject that the tool applies the policy
	// to.
	SubjectKind specv1.SubjectKind

	// MachineID is the machine id of an endpoint. The header must carry it.
	// The tool does not know its server endpoint id, so Decode does not check
	// the subject id of an endpoint.
	MachineID string

	// ProjectID is the id of a project. The subject id must equal it.
	ProjectID string
}

// Header holds the values of the header of a resolved policy.
type Header struct {
	Tenant                string
	SubjectKind           specv1.SubjectKind
	SubjectID             string
	MachineID             string
	Revision              uint64
	DefaultPolicyRevision uint64
	IssuedAt              time.Time

	// NextExpiryAt is the zero time when no entry expires.
	NextExpiryAt time.Time

	MatchedOverrides []MatchedOverride
}

// MatchedOverride is a policy override that matches the subject.
type MatchedOverride struct {
	ID       string
	Name     string
	Priority uint32
}

// SourceKey names one field of a resolved policy, or one item of a repeated
// setting.
type SourceKey struct {
	// Path is the contract path, as in "package_admission.cooldown.min_age".
	Path string

	// ItemKey names one item of a repeated setting. It is empty for a field
	// that one layer set as a whole.
	ItemKey string
}

// Source is the layer that set a field.
type Source struct {
	Layer recordv1.Layer

	// OverrideID is the policy override that set the field, when Layer is
	// LAYER_OVERRIDE.
	OverrideID string
}

// Projection is a resolved policy in the plain form that a tool applies.
type Projection struct {
	Header Header

	// Values maps the contract path of each setting to its value. A value is
	// a time.Duration, a time.Time, an int64, a float64, a bool, a string,
	// []byte, an Enum, or a []any of these for a repeated setting. A setting
	// that the policy does not set is not in the map.
	Values map[string]any

	// Sources maps each field to the layer that set it.
	Sources map[SourceKey]Source

	// Rules are the package rules in normalized form. An expired rule stays
	// in the list: match checks the expiry at each decision.
	Rules []match.Rule

	// Exceptions are the fields that the tool does not apply as the policy
	// says.
	Exceptions []Exception

	// FailedCapabilities names each capability, as "package_admission", with
	// a value that fails its contract limits. Values holds none of its
	// settings, until Restore takes them from a previous policy.
	FailedCapabilities []string

	// Restored names each failed capability that Restore took from a previous
	// policy.
	Restored []string
}

// Decode decodes the bytes of a resolved policy, checks the header against
// expect and projects the policy. It returns ErrDecode for bytes that do not
// decode and ErrHeader for a header that fails its check. Each other problem
// is an Exception of the Projection.
func Decode(raw []byte, expect Expect) (*Projection, error) {
	var policy specv1.ResolvedPolicy
	if err := proto.Unmarshal(raw, &policy); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecode, err)
	}

	header, err := checkHeader(&policy, expect)
	if err != nil {
		return nil, err
	}

	// The contract requires the spec. A document with none is not a policy
	// that the server wrote, so it must not replace the policy that the tool
	// applies now.
	if !policy.HasSpec() {
		return nil, fmt.Errorf("%w: no spec", ErrDecode)
	}

	p := &Projection{
		Header:  header,
		Values:  map[string]any{},
		Sources: map[SourceKey]Source{},
	}

	p.Exceptions = append(p.Exceptions, unknownFields(&policy)...)
	p.projectRules(policy.GetSpec().GetCore().GetPackageRules())
	p.projectSettings(policy.GetSpec())

	for _, fs := range policy.GetFieldSources() {
		p.Sources[SourceKey{Path: fs.GetPath(), ItemKey: fs.GetItemKey()}] = Source{
			Layer:      fs.GetLayer(),
			OverrideID: fs.GetOverrideId(),
		}
	}

	return p, nil
}

// Restore takes each failed capability of p from prev: its values and its
// sources. prev is the projection of the previous policy that the tool
// applied. A capability that failed in prev too comes from prev only when
// prev restored it, so a value survives a failure in two policies in a row.
// Restore returns the capabilities that it took. A failed capability that it
// does not return has no value, and the lower layer of the tool applies.
func (p *Projection) Restore(prev *Projection) []string {
	if prev == nil {
		return nil
	}

	var restored []string
	for _, capability := range p.FailedCapabilities {
		if slices.Contains(p.Restored, capability) {
			continue
		}

		if slices.Contains(prev.FailedCapabilities, capability) && !slices.Contains(prev.Restored, capability) {
			continue
		}

		prefix := capability + "."
		for path, v := range prev.Values {
			if strings.HasPrefix(path, prefix) {
				p.Values[path] = v
			}
		}

		for key, s := range prev.Sources {
			if strings.HasPrefix(key.Path, prefix) {
				p.Sources[key] = s
			}
		}

		restored = append(restored, capability)
	}

	p.Restored = append(p.Restored, restored...)
	return restored
}
