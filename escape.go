// Copyright (c) the go-ruby-builder/builder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"fmt"
	"strings"
)

// sprint is fmt.Sprint, factored out so valueToString's default branch stays a
// one-liner and the fmt dependency is confined here.
func sprint(v any) string { return fmt.Sprint(v) }

// escapeText escapes a string for XML character data exactly as Builder's
// _escape (XChar.encode) does: only '&', '<' and '>' become predefined
// entities, and characters that are not legal XML (control chars other than
// \t \n \r) are replaced with U+FFFD. Quotes and apostrophes are left intact —
// matching MRI Builder, which does not escape them in text.
func escapeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			if validXMLChar(r) {
				b.WriteRune(r)
			} else {
				b.WriteRune('�')
			}
		}
	}
	return b.String()
}

// escapeAttr escapes a string for an XML attribute value exactly as Builder's
// _escape_attribute does: _escape first (so '&', '<', '>' become entities and
// invalid chars are replaced), then '\n' -> &#10;, '\r' -> &#13;, and '"' ->
// &quot;. A literal tab and an apostrophe are left intact, as in MRI Builder.
func escapeAttr(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\n':
			b.WriteString("&#10;")
		case '\r':
			b.WriteString("&#13;")
		case '"':
			b.WriteString("&quot;")
		default:
			if validXMLChar(r) {
				b.WriteRune(r)
			} else {
				b.WriteRune('�')
			}
		}
	}
	return b.String()
}

// validXMLChar reports whether r is permitted in an XML 1.0 document, per
// https://www.w3.org/TR/REC-xml/#charsets — the set Builder's XChar.VALID
// encodes. Everything outside it is replaced with the Unicode replacement char.
func validXMLChar(r rune) bool {
	switch {
	case r == 0x9, r == 0xA, r == 0xD:
		return true
	case r >= 0x20 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0x10FFFF:
		return true
	default:
		return false
	}
}
