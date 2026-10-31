package project

import (
	"fmt"
	"strconv"

	recordv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/record/v1"
	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ExceptionKind says why a tool does not apply a field as the policy says.
type ExceptionKind int

const (
	// ExceptionUnknownField is a field that the tool does not know.
	ExceptionUnknownField ExceptionKind = iota + 1

	// ExceptionUnknownEnumValue is an enum value that the tool does not
	// know. For an ordered enum, the tool applies the strictest value that
	// it knows.
	ExceptionUnknownEnumValue

	// ExceptionDroppedRule is a package rule that the tool dropped.
	ExceptionDroppedRule

	// ExceptionInvalidValue is a value that fails its contract limits. Its
	// capability fails.
	ExceptionInvalidValue

	// ExceptionUnsupported is a setting whose shape the value tree cannot
	// hold, such as a map. Its capability fails.
	ExceptionUnsupported
)

// Exception is one field of the resolved policy that the tool does not apply
// as the policy says.
type Exception struct {
	// Path is the contract path. A segment that the tool does not know is its
	// field number, as in "package_admission.7".
	Path string

	Kind ExceptionKind

	// RuleID is the id of the package rule, for an exception of a rule.
	RuleID string

	// Detail is for people.
	Detail string
}

// Reason maps the kind to the reason of an ApplyReport. The contract has no
// reason for a value that fails its limits, so ExceptionInvalidValue maps to
// FIELD_EXCEPTION_REASON_UNSPECIFIED.
func (k ExceptionKind) Reason() recordv1.FieldExceptionReason {
	switch k {
	case ExceptionUnknownField:
		return recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNKNOWN_FIELD
	case ExceptionUnknownEnumValue:
		return recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNKNOWN_ENUM_VALUE
	case ExceptionDroppedRule:
		return recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_DROPPED_RULE
	case ExceptionUnsupported:
		return recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNSUPPORTED
	default:
		return recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNSPECIFIED
	}
}

// FieldException returns the exception in the form of an ApplyReport.
func (e Exception) FieldException() *recordv1.FieldException {
	detail := e.Detail
	if e.RuleID != "" {
		detail = fmt.Sprintf("rule %s: %s", e.RuleID, e.Detail)
	}

	return recordv1.FieldException_builder{
		Path:   e.Path,
		Reason: e.Kind.Reason(),
		Detail: detail,
	}.Build()
}

// The contract paths start at the PolicySpec. The other parts of a resolved
// policy get these names, so they never look like a contract path.
const (
	pathResolvedPolicy = "resolved_policy"
	pathPackageRules   = "core.package_rules"
)

// unknownFields reports each unknown field of the resolved policy by its
// path. The package rules report their own unknown fields, because a trust
// entry with one is dropped.
func unknownFields(policy *specv1.ResolvedPolicy) []Exception {
	var out []Exception

	root := policy.ProtoReflect()
	for _, num := range unknownNumbers(root) {
		out = append(out, unknownFieldException(join(pathResolvedPolicy, num), ""))
	}

	root.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		path := string(fd.Name())
		if fd.Number() == specFieldNumber {
			path = ""
		}

		walkUnknown(fd, v, path, &out)
		return true
	})

	return out
}

var specFieldNumber = (&specv1.ResolvedPolicy{}).ProtoReflect().Descriptor().Fields().ByName("spec").Number()

func walkUnknown(fd protoreflect.FieldDescriptor, v protoreflect.Value, path string, out *[]Exception) {
	if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
		return
	}

	if path == pathPackageRules {
		return
	}

	switch {
	case fd.IsList():
		list := v.List()
		reported := map[string]bool{}
		for i := range list.Len() {
			for _, e := range messageUnknown(list.Get(i).Message(), path) {
				if !reported[e.Path] {
					reported[e.Path] = true
					*out = append(*out, e)
				}
			}
		}

	case fd.IsMap():
		v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
			walkUnknown(fd.MapValue(), mv, path, out)
			return true
		})

	default:
		*out = append(*out, messageUnknown(v.Message(), path)...)
	}
}

// messageUnknown reports the unknown fields of m and of every message in it.
func messageUnknown(m protoreflect.Message, path string) []Exception {
	var out []Exception
	for _, num := range unknownNumbers(m) {
		out = append(out, unknownFieldException(join(path, num), ""))
	}

	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		walkUnknown(fd, v, join(path, string(fd.Name())), &out)
		return true
	})

	return out
}

func unknownFieldException(path, ruleID string) Exception {
	return Exception{Path: path, Kind: ExceptionUnknownField, RuleID: ruleID, Detail: "the tool does not know the field"}
}

// unknownNumbers returns the field number of each unknown field of m, once
// each, in the order of the bytes. Bytes that do not parse end the list:
// proto.Unmarshal accepted them, so this does not happen in practice.
func unknownNumbers(m protoreflect.Message) []string {
	raw := m.GetUnknown()
	seen := map[protowire.Number]bool{}

	var nums []string
	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			break
		}
		raw = raw[n:]

		n = protowire.ConsumeFieldValue(num, typ, raw)
		if n < 0 {
			break
		}
		raw = raw[n:]

		if !seen[num] {
			seen[num] = true
			nums = append(nums, strconv.Itoa(int(num)))
		}
	}

	return nums
}

func join(path, name string) string {
	if path == "" {
		return name
	}

	if name == "" {
		return path
	}

	return path + "." + name
}
