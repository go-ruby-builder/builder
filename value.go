// Copyright (c) the go-ruby-builder/builder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"math/big"
	"strconv"
	"strings"
)

// Value is any Ruby value the builder accepts as element content, text, or an
// attribute value. The rbgo host maps its own object.Value to and from this
// small, fixed set; the emitter itself never touches a Ruby runtime.
//
// The concrete shapes that carry meaning are:
//
//	nil                 -> no content (self-closing tag) / a nil attribute value
//	string              -> text, XML-escaped
//	Symbol              -> a bare name (Ruby to_s), XML-escaped like a string
//	bool                -> "true" / "false"
//	int, int64, ...     -> decimal integer
//	*big.Int            -> arbitrary-precision integer
//	float64, float32    -> Ruby Float#to_s
//	Raw                 -> pre-formatted markup inserted verbatim (the `<<` path)
//
// Any other Go value is stringified with valueToString's fallback (Sprint), so
// the host can hand through a type it has already rendered.
type Value = any

// Symbol is a Ruby Symbol. As element content or an attribute value it renders
// as its bare name (Ruby's Symbol#to_s), then XML-escaped like a string.
type Symbol string

// Raw is markup inserted verbatim, with no escaping — the Go equivalent of
// Ruby's `xml << "<i>raw</i>"`. Text!/content strings are always escaped; wrap a
// string in Raw to bypass that.
type Raw string

// valueToString renders a scalar Value to the string Ruby's Builder would obtain
// via to_s before escaping. It reports ok=false for nil so callers can decide
// between "no content" and an empty string.
func valueToString(v Value) (s string, ok bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case string:
		return x, true
	case Symbol:
		return string(x), true
	case Raw:
		return string(x), true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case int:
		return strconv.FormatInt(int64(x), 10), true
	case int8:
		return strconv.FormatInt(int64(x), 10), true
	case int16:
		return strconv.FormatInt(int64(x), 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case uint:
		return strconv.FormatUint(uint64(x), 10), true
	case uint8:
		return strconv.FormatUint(uint64(x), 10), true
	case uint16:
		return strconv.FormatUint(uint64(x), 10), true
	case uint32:
		return strconv.FormatUint(uint64(x), 10), true
	case uint64:
		return strconv.FormatUint(x, 10), true
	case *big.Int:
		return x.String(), true
	case float64:
		return rubyFloat(x), true
	case float32:
		return rubyFloat(float64(x)), true
	case fmtStringer:
		return x.String(), true
	default:
		return sprint(v), true
	}
}

// fmtStringer mirrors fmt.Stringer without importing fmt at the value layer's
// hot path; a host value that already renders itself is honoured.
type fmtStringer interface{ String() string }

// rubyFloat formats a float the way Ruby's Float#to_s does for the common cases
// that appear in markup: an integral float keeps a trailing ".0", and other
// values use the shortest round-tripping decimal.
func rubyFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eEnN") {
		s += ".0"
	}
	return s
}
