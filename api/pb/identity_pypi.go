package pb

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// pep440Pattern is the PEP 440 grammar in its permissive form, the one
// packaging.version.Version accepts. Case, separators, leading zeros and the
// long pre-release spellings are all allowed here and folded below.
var pep440Pattern = regexp.MustCompile(`(?i)^v?(?:(?P<epoch>[0-9]+)!)?` +
	`(?P<release>[0-9]+(?:\.[0-9]+)*)` +
	`(?:[._-]?(?P<pre>a|b|c|rc|alpha|beta|pre|preview)[._-]?(?P<preN>[0-9]+)?)?` +
	`(?:-(?P<postImplicit>[0-9]+)|[._-]?(?P<post>post|rev|r)[._-]?(?P<postN>[0-9]+)?)?` +
	`(?:[._-]?(?P<dev>dev)[._-]?(?P<devN>[0-9]+)?)?` +
	`(?:\+(?P<local>[a-z0-9]+(?:[._-][a-z0-9]+)*))?$`)

// canonicalPypiVersion folds a version string to the form
// packaging.utils.canonicalize_version produces: no leading v, no zero epoch,
// no leading zeros in a number, no trailing zero release segments, lower case,
// one spelling per pre, post and dev segment, dots between local segments.
// PyPI resolves a release by this form, so two strings with one canonical form
// name one release. A string the grammar rejects comes back unchanged with
// false, so the fold is total.
func canonicalPypiVersion(version string) (string, bool) {
	trimmed := strings.TrimSpace(version)
	if !isASCII(trimmed) {
		// PEP 440 is an ASCII grammar. Go's case-insensitive match folds
		// Unicode too, so the Kelvin sign would match [a-z] and collide
		// with the ASCII k that packaging accepts.
		return version, false
	}

	matches := pep440Pattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return version, false
	}

	part := func(name string) string {
		return strings.ToLower(matches[pep440Pattern.SubexpIndex(name)])
	}

	release := strings.Split(part("release"), ".")
	for i := range release {
		release[i] = pep440Number(release[i])
	}
	for len(release) > 1 && release[len(release)-1] == "0" {
		release = release[:len(release)-1]
	}
	result := strings.Join(release, ".")

	if epoch := pep440Number(part("epoch")); epoch != "0" {
		result = epoch + "!" + result
	}

	if pre := part("pre"); pre != "" {
		switch pre {
		case "alpha":
			pre = "a"
		case "beta":
			pre = "b"
		case "c", "pre", "preview":
			pre = "rc"
		}
		result += pre + pep440Number(part("preN"))
	}

	if implicit := part("postImplicit"); implicit != "" {
		result += ".post" + pep440Number(implicit)
	} else if part("post") != "" {
		result += ".post" + pep440Number(part("postN"))
	}

	if part("dev") != "" {
		result += ".dev" + pep440Number(part("devN"))
	}

	if local := part("local"); local != "" {
		segments := strings.FieldsFunc(local, func(r rune) bool { return r == '.' || r == '_' || r == '-' })
		for i, segment := range segments {
			if strings.Trim(segment, "0123456789") == "" {
				segments[i] = pep440Number(segment)
			}
		}
		result += "+" + strings.Join(segments, ".")
	}

	return result, true
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func pep440Number(number string) string {
	number = strings.TrimLeft(number, "0")
	if number == "" {
		return "0"
	}
	return number
}
