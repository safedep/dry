package project

import (
	"slices"
	"strings"

	optionsv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/options/v1"
	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Field is one field of the policy contract that carries a field contract.
type Field struct {
	// Path is the contract path, as in "package_admission.cooldown.min_age".
	Path string

	// Capability is the first segment of the path, as in
	// "package_admission".
	Capability string

	Merge              optionsv1.Merge
	DefaultEnforcement optionsv1.Enforcement
	Stricter           optionsv1.Stricter

	// BindingRequired is true when every tool that implements the capability
	// must map the field in its binding table.
	BindingRequired bool
}

// Fields returns each field of the policy contract that this build of the
// API knows and that carries a field contract, in path order. It reads the
// descriptor, so the contract stays the one source of field metadata. A
// tool test uses it to check that its binding table maps each field marked
// binding_required.
func Fields() []Field {
	var out []Field
	walkFields((&specv1.PolicySpec{}).ProtoReflect().Descriptor(), "", map[protoreflect.FullName]bool{}, &out)

	slices.SortFunc(out, func(a, b Field) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func walkFields(md protoreflect.MessageDescriptor, path string, seen map[protoreflect.FullName]bool, out *[]Field) {
	if seen[md.FullName()] {
		return
	}
	seen[md.FullName()] = true
	defer delete(seen, md.FullName())

	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		fieldPath := join(path, string(fd.Name()))

		if !proto.HasExtension(fd.Options(), optionsv1.E_Field) {
			if fd.Kind() == protoreflect.MessageKind && !fd.IsMap() && !isLeafMessage(fd.Message()) {
				walkFields(fd.Message(), fieldPath, seen, out)
			}
			continue
		}

		if contract, ok := proto.GetExtension(fd.Options(), optionsv1.E_Field).(*optionsv1.FieldContract); ok {
			capability, _, _ := strings.Cut(fieldPath, ".")
			*out = append(*out, Field{
				Path:               fieldPath,
				Capability:         capability,
				Merge:              contract.GetMerge(),
				DefaultEnforcement: contract.GetDefaultEnforcement(),
				Stricter:           contract.GetStricter(),
				BindingRequired:    contract.GetBindingRequired(),
			})
		}
	}
}
