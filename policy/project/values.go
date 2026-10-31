package project

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	optionsv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/options/v1"
	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Enum is the number of an enum value in the value tree.
type Enum int32

var (
	durationName  = (&durationpb.Duration{}).ProtoReflect().Descriptor().FullName()
	timestampName = (&timestamppb.Timestamp{}).ProtoReflect().Descriptor().FullName()
	coreFieldName = protoreflect.Name("core")
)

// projectSettings fills the value tree from each capability of the spec,
// except core: package rules have their own projection rules. A capability
// whose value fails its contract limits, or does not fit the value tree,
// fails as a whole.
func (p *Projection) projectSettings(spec *specv1.PolicySpec) {
	if spec == nil {
		return
	}

	spec.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.Name() == coreFieldName || fd.Kind() != protoreflect.MessageKind || fd.IsList() || fd.IsMap() {
			return true
		}

		capability := string(fd.Name())
		values := map[string]any{}
		w := settingsWalker{values: values}
		w.message(v.Message(), capability)
		w.validate(v.Message(), capability)

		p.Exceptions = append(p.Exceptions, w.exceptions...)
		if w.failed {
			p.FailedCapabilities = append(p.FailedCapabilities, capability)
			return true
		}

		for path, value := range values {
			p.Values[path] = value
		}

		return true
	})

	slices.Sort(p.FailedCapabilities)
}

// settingsWalker collects the values of one capability.
type settingsWalker struct {
	values     map[string]any
	exceptions []Exception
	failed     bool
}

func (w *settingsWalker) fail(path string, kind ExceptionKind, detail string) {
	w.failed = true
	w.exceptions = append(w.exceptions, Exception{Path: path, Kind: kind, Detail: detail})
}

func (w *settingsWalker) message(m protoreflect.Message, path string) {
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		w.field(fd, v, join(path, string(fd.Name())))
		return true
	})
}

func (w *settingsWalker) field(fd protoreflect.FieldDescriptor, v protoreflect.Value, path string) {
	switch {
	case fd.IsMap():
		w.fail(path, ExceptionUnsupported, "the value tree cannot hold a map")

	case fd.IsList():
		if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
			if !isLeafMessage(fd.Message()) {
				w.fail(path, ExceptionUnsupported, "the value tree cannot hold a list of messages")
				return
			}
		}

		list := v.List()
		items := make([]any, 0, list.Len())
		for i := range list.Len() {
			item, ok := w.scalar(fd, list.Get(i), path)
			if !ok {
				return
			}
			items = append(items, item)
		}
		w.values[path] = items

	case (fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind) && !isLeafMessage(fd.Message()):
		w.message(v.Message(), path)

	default:
		if value, ok := w.scalar(fd, v, path); ok {
			w.values[path] = value
		}
	}
}

func isLeafMessage(md protoreflect.MessageDescriptor) bool {
	return md.FullName() == durationName || md.FullName() == timestampName
}

// scalar converts one value. It returns false when the value fails, and then
// it has recorded the failure.
func (w *settingsWalker) scalar(fd protoreflect.FieldDescriptor, v protoreflect.Value, path string) (any, bool) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return v.Bool(), true

	case protoreflect.StringKind:
		return v.String(), true

	case protoreflect.BytesKind:
		return slices.Clone(v.Bytes()), true

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return v.Int(), true

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if v.Uint() > math.MaxInt64 {
			w.fail(path, ExceptionInvalidValue, fmt.Sprintf("value %d does not fit an int64", v.Uint()))
			return nil, false
		}
		return int64(v.Uint()), true

	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return v.Float(), true

	case protoreflect.EnumKind:
		return w.enum(fd.Enum(), v.Enum(), path)

	case protoreflect.MessageKind, protoreflect.GroupKind:
		return w.leafMessage(v.Message(), path)

	default:
		w.fail(path, ExceptionUnsupported, fmt.Sprintf("the value tree cannot hold a %s", fd.Kind()))
		return nil, false
	}
}

func (w *settingsWalker) leafMessage(m protoreflect.Message, path string) (any, bool) {
	switch msg := m.Interface().(type) {
	case *durationpb.Duration:
		if err := msg.CheckValid(); err != nil {
			w.fail(path, ExceptionInvalidValue, err.Error())
			return nil, false
		}
		return msg.AsDuration(), true

	case *timestamppb.Timestamp:
		if err := msg.CheckValid(); err != nil {
			w.fail(path, ExceptionInvalidValue, err.Error())
			return nil, false
		}
		return msg.AsTime(), true

	default:
		w.fail(path, ExceptionUnsupported, fmt.Sprintf("the value tree cannot hold a %s", m.Descriptor().FullName()))
		return nil, false
	}
}

// enum maps an unknown value of an ordered enum to the strictest value that
// the tool knows. An unknown value of an enum with no order has no safe
// value, so its capability fails.
func (w *settingsWalker) enum(ed protoreflect.EnumDescriptor, n protoreflect.EnumNumber, path string) (any, bool) {
	if ed.Values().ByNumber(n) != nil {
		return Enum(n), true
	}

	if !isOrdered(ed) {
		w.fail(path, ExceptionUnknownEnumValue, fmt.Sprintf("unknown value %d of %s, which has no order", n, ed.FullName()))
		return nil, false
	}

	strictest, ok := strictestValue(ed)
	if !ok {
		w.fail(path, ExceptionUnknownEnumValue, fmt.Sprintf("unknown value %d of %s, which has no known value", n, ed.FullName()))
		return nil, false
	}

	w.exceptions = append(w.exceptions, Exception{
		Path:   path,
		Kind:   ExceptionUnknownEnumValue,
		Detail: fmt.Sprintf("unknown value %d, the tool applies the strictest known value %s", n, ed.Values().ByNumber(strictest).Name()),
	})

	return Enum(strictest), true
}

func isOrdered(ed protoreflect.EnumDescriptor) bool {
	ordered, _ := proto.GetExtension(ed.Options(), optionsv1.E_Ordered).(bool)
	return ordered
}

// strictestValue returns the known value with the highest strictness. A
// value with no strictness, such as the zero value, is never the result.
func strictestValue(ed protoreflect.EnumDescriptor) (protoreflect.EnumNumber, bool) {
	var (
		best      protoreflect.EnumNumber
		bestScore int32
		found     bool
	)

	values := ed.Values()
	for i := range values.Len() {
		vd := values.Get(i)
		contract, _ := proto.GetExtension(vd.Options(), optionsv1.E_Value).(*optionsv1.ValueContract)
		score := contract.GetStrictness()
		if score <= 0 {
			continue
		}

		if !found || score > bestScore {
			best, bestScore, found = vd.Number(), score, true
		}
	}

	return best, found
}

// validate applies the contract limits of the capability, the protovalidate
// rules of its message.
func (w *settingsWalker) validate(m protoreflect.Message, capability string) {
	err := protovalidate.Validate(m.Interface())
	if err == nil {
		return
	}

	var verr *protovalidate.ValidationError
	if !errors.As(err, &verr) {
		w.fail(capability, ExceptionInvalidValue, fmt.Sprintf("cannot check the contract limits: %v", err))
		return
	}

	for _, v := range verr.Violations {
		path := capability
		if fp := protovalidate.FieldPathString(v.Proto.GetField()); fp != "" {
			path = join(capability, fp)
		}

		w.fail(path, ExceptionInvalidValue, strings.TrimSpace(v.Proto.GetMessage()))
	}
}
