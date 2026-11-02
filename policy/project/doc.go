// Package project decodes a resolved policy of SafeDep Cloud and projects it
// onto the fields that a tool knows.
//
// Decode checks the header, applies the projection rules of the policy
// contract, and returns a Projection: the header values, a plain value tree
// of the settings, the layer that set each field, the package rules as
// match.Rule values, and the exceptions. A tool reads only the Projection, so
// its config loader needs no import of the API contract.
//
// The projection rules fail closed:
//   - Only bytes that do not decode, a document with no spec, or a header,
//     tenant or subject failure reject the whole document.
//   - An unknown field is reported by its contract path, with the field
//     number for the unknown part, as in "package_admission.7".
//   - An unknown value of an ordered enum becomes the strictest value that
//     the tool knows.
//   - A trust entry with an unknown field is dropped. A block entry with an
//     unknown field is kept. match.Normalize applies the other rules of a
//     package rule.
//   - A value that fails its contract limits fails its capability. The
//     caller takes that capability from the previous policy with Restore.
package project
