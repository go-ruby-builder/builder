// Copyright (c) the go-ruby-builder/builder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"math"
	"math/big"
	"testing"
)

// stringerT is a host type that renders itself, exercising valueToString's
// fmtStringer branch.
type stringerT struct{ s string }

func (s stringerT) String() string { return s.s }

// TestValueToStringScalars covers every numeric / scalar branch of
// valueToString, including the Stringer and the Sprint fallback.
func TestValueToStringScalars(t *testing.T) {
	bi, _ := new(big.Int).SetString("42", 10)
	cases := []struct {
		v    Value
		want string
	}{
		{"s", "s"},
		{Symbol("y"), "y"},
		{Raw("<r/>"), "<r/>"},
		{true, "true"},
		{false, "false"},
		{int(1), "1"},
		{int8(2), "2"},
		{int16(3), "3"},
		{int32(4), "4"},
		{int64(5), "5"},
		{uint(6), "6"},
		{uint8(7), "7"},
		{uint16(8), "8"},
		{uint32(9), "9"},
		{uint64(10), "10"},
		{bi, "42"},
		{float64(1.5), "1.5"},
		{float32(2.5), "2.5"},
		{float64(3), "3.0"},         // integral float keeps .0
		{1e21, "1.0e+21"},           // exponential form keeps ".0" in the mantissa
		{1.5e21, "1.5e+21"},         // exponential mantissa already has a dot
		{math.Inf(1), "Infinity"},   // Ruby Float::INFINITY
		{math.Inf(-1), "-Infinity"}, // Ruby -Float::INFINITY
		{math.NaN(), "NaN"},         // Ruby Float::NAN
		{stringerT{"host"}, "host"}, // fmtStringer branch
		{[]int{1, 2}, "[1 2]"},      // Sprint fallback
	}
	for _, c := range cases {
		if got, _ := valueToString(c.v); got != c.want {
			t.Errorf("valueToString(%#v) = %q, want %q", c.v, got, c.want)
		}
	}
	// nil reports ok=false.
	if s, ok := valueToString(nil); ok || s != "" {
		t.Errorf("valueToString(nil) = (%q, %v)", s, ok)
	}
}

// TestValueKeyFallback covers valueKey when valueToString yields ok=false (nil),
// which returns an empty key.
func TestValueKeyFallback(t *testing.T) {
	if got := valueKey(nil); got != "" {
		t.Errorf("valueKey(nil) = %q, want empty", got)
	}
	if got := valueKey(Symbol("id")); got != "id" {
		t.Errorf("valueKey(Symbol) = %q", got)
	}
}

// TestValidXMLChar exercises each range branch of validXMLChar.
func TestValidXMLChar(t *testing.T) {
	valid := []rune{0x9, 0xA, 0xD, 0x20, 0xD7FF, 0xE000, 0xFFFD, 0x10000, 0x10FFFF}
	for _, r := range valid {
		if !validXMLChar(r) {
			t.Errorf("validXMLChar(%#x) = false, want true", r)
		}
	}
	invalid := []rune{0x0, 0x1, 0x8, 0xB, 0xC, 0xE, 0x1F, 0xD800, 0xFFFE, 0xFFFF, 0x110000}
	for _, r := range invalid {
		if validXMLChar(r) {
			t.Errorf("validXMLChar(%#x) = true, want false", r)
		}
	}
}

// TestAttrsGetMiss covers Attrs.get when the key is absent.
func TestAttrsGetMiss(t *testing.T) {
	a := Attrs{{Key: "x", Value: 1}}
	if _, ok := a.get("nope"); ok {
		t.Error("get(missing) reported ok")
	}
	if v, ok := a.get("x"); !ok || v != 1 {
		t.Errorf("get(x) = (%v, %v)", v, ok)
	}
}

// TestInstructEncodingTracked covers the encoding-tracking branch: instruct!
// :xml records the (possibly overridden) encoding, whose lower-cased form is
// kept for later escaping decisions.
func TestInstructEncodingTracked(t *testing.T) {
	x := New()
	x.Instruct("xml", Of("encoding", "ISO-8859-1"))
	if x.encoding != "iso-8859-1" {
		t.Errorf("encoding = %q", x.encoding)
	}
}

// TestDeclareBlockXmlMarkupArg covers Declare's func(*XmlMarkup) block branch
// (the golden uses func()).
func TestDeclareBlockXmlMarkupArg(t *testing.T) {
	x := New(WithIndent(2))
	x.Declare("DOCTYPE", Sym("html"), func(b *XmlMarkup) { b.Declare("ELEMENT", Sym("br"), "EMPTY") })
	want := "<!DOCTYPE html [\n  <!ELEMENT br \"EMPTY\">\n]>\n"
	if x.Target() != want {
		t.Errorf("got %q want %q", x.Target(), want)
	}
}
