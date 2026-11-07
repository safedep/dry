// Package spdxlicense reads the license of a package as an SPDX license
// expression (SPDX 2.3 Annex D), and checks it against allow and deny
// lists. It parses the expression itself, in linear time, and takes the
// license and exception ids, with the OSI and FSF flags, from the SPDX
// License List data that it embeds.
//
// The package holds the one copy of the SPDX rules of the cloud policy. The
// pull request check and vet both call it, so a license gate decides the
// same way in each tool:
//
//   - An SPDX id compares in any case. A deprecated id compares as its
//     current id: GPL-2.0 is GPL-2.0-only, and GPL-2.0+ and
//     GPL-2.0-only+ are GPL-2.0-or-later. A LicenseRef compares exactly.
//   - An entry matches a license of the same form only, so GPL-2.0-only does
//     not match GPL-2.0-only WITH Classpath-exception-2.0.
//   - An or-later license fails a deny list when the list names it or each
//     version that it allows. It passes an allow list when the list names it
//     or one of those versions.
//   - A package passes when one choice of an OR expression fails no list.
//     Every part of an AND must pass. The license values of one package join
//     with AND. A denied or-later term stays denied, and the allow list
//     cannot pass it through one of its later versions.
//
// NewGate and Gate.Evaluate apply a license gate of a policy: the deny list,
// the allow list and the action on an unknown license. ValidateEntry checks
// one entry of a list of the policy contract. NewPolicy and Check are the
// lower layer that the vet license control uses, with the vet set names.
//
// list.json holds the license ids of the SPDX License List with their OSI
// and FSF flags, and the active exception ids. go generate copies them from
// a pinned commit of github.com/spdx/license-list-data. The SPDX License
// List is under CC-BY-3.0, and NOTICE gives its version, source and author.
package spdxlicense
