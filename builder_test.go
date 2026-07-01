// Copyright (c) the go-ruby-builder/builder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"math/big"
	"strings"
	"testing"
)

// goldens is a table of deterministic (ruby-free) vectors: a builder program and
// the exact markup builder 3.3.0 (MRI, Ruby >= 4.0) produces for it. It alone
// holds coverage at 100%, so the qemu cross-arch and Windows lanes pass the gate
// without a ruby present. Each want was captured from the gem (see oracle_test).
var goldens = []struct {
	name string
	fn   func() string
	want string
}{
	{"nested", func() string {
		x := New()
		x.Tag("person", func() { x.Tag("name", "Alice"); x.Tag("age", 30) })
		return x.Target()
	}, "<person><name>Alice</name><age>30</age></person>"},

	{"attrs", func() string {
		x := New()
		x.Tag("person", Of("id", 1, "class", "vip"), func() { x.Tag("name", "Bob") })
		return x.Target()
	}, `<person id="1" class="vip"><name>Bob</name></person>`},

	{"empty", func() string { x := New(); x.Tag("br"); return x.Target() }, "<br/>"},

	{"empty-attr", func() string {
		x := New()
		x.Tag("img", Of("src", "a.png"))
		return x.Target()
	}, `<img src="a.png"/>`},

	{"empty-string-body", func() string { x := New(); x.Tag("a", ""); return x.Target() }, "<a></a>"},

	{"text-escape", func() string {
		x := New()
		x.Tag("p", "a < b & c > d \" ' ")
		return x.Target()
	}, `<p>a &lt; b &amp; c &gt; d " ' </p>`},

	{"attr-escape", func() string {
		x := New()
		x.Tag("p", Of("t", `a<b&c>"'`))
		return x.Target()
	}, `<p t="a&lt;b&amp;c&gt;&quot;'"/>`},

	{"attr-whitespace", func() string {
		x := New()
		x.Tag("a", Of("t", "x\ny\tz\rw"))
		return x.Target()
	}, "<a t=\"x&#10;y\tz&#13;w\"/>"},

	{"text-bang", func() string {
		x := New()
		x.Tag("p", func() { x.Text("hi < there") })
		return x.Target()
	}, "<p>hi &lt; there</p>"},

	{"text-block-arg", func() string {
		x := New()
		x.Tag("a", func(b *XmlMarkup) { b.Text("x") })
		return x.Target()
	}, "<a>x</a>"},

	{"append-raw", func() string {
		x := New()
		x.Tag("a", func() { x.Append("<raw/>") })
		return x.Target()
	}, "<a><raw/></a>"},

	{"append-raw-type", func() string {
		x := New()
		x.Tag("a", func() { x.Append(Raw("<b/>")) })
		return x.Target()
	}, "<a><b/></a>"},

	{"multi-text", func() string {
		x := New()
		x.Tag("a", "foo", "bar", "baz")
		return x.Target()
	}, "<a>foobarbaz</a>"},

	{"multi-root", func() string { x := New(); x.Tag("a"); x.Tag("b"); return x.Target() }, "<a/><b/>"},

	{"tag-ns", func() string { x := New(); x.Tag("my:elem", "v"); return x.Target() }, "<my:elem>v</my:elem>"},

	{"ns-helper", func() string {
		x := New()
		x.NS("soap", "Envelope", Of("xmlns", "http://x"))
		return x.Target()
	}, `<soap:Envelope xmlns="http://x"/>`},

	{"nil-attr", func() string { x := New(); x.Tag("v", Of("a", nil)); return x.Target() }, `<v a=""/>`},

	{"symbol-attr-value", func() string {
		x := New()
		x.Tag("a", Of("k", Symbol("sym")))
		return x.Target()
	}, `<a k="sym"/>`},

	{"bool-content", func() string { x := New(); x.Tag("a", true); return x.Target() }, "<a>true</a>"},
	{"false-content", func() string { x := New(); x.Tag("a", false); return x.Target() }, "<a>false</a>"},
	{"float-content", func() string { x := New(); x.Tag("a", 3.14); return x.Target() }, "<a>3.14</a>"},
	{"int-float-content", func() string { x := New(); x.Tag("a", 2.0); return x.Target() }, "<a>2.0</a>"},
	{"float32-content", func() string { x := New(); x.Tag("a", float32(1.5)); return x.Target() }, "<a>1.5</a>"},
	{"num-content", func() string { x := New(); x.Tag("a", 42); return x.Target() }, "<a>42</a>"},
	{"symbol-content", func() string { x := New(); x.Tag("a", Symbol("hi")); return x.Target() }, "<a>hi</a>"},

	{"bignum-content", func() string {
		bi, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
		x := New()
		x.Tag("a", bi)
		return x.Target()
	}, "<a>123456789012345678901234567890</a>"},

	{"element-helper", func() string {
		x := New()
		x.Element("a", Of("id", 1), "hi")
		return x.Target()
	}, `<a id="1">hi</a>`},

	{"element-helper-selfclose", func() string {
		x := New()
		x.Element("br", nil, nil)
		return x.Target()
	}, "<br/>"},

	{"element-helper-attrs-selfclose", func() string {
		x := New()
		x.Element("img", Of("src", "a.png"), nil)
		return x.Target()
	}, `<img src="a.png"/>`},

	{"element-helper-content-noattrs", func() string {
		x := New()
		x.Element("a", nil, "hi")
		return x.Target()
	}, "<a>hi</a>"},

	{"cdata", func() string { x := New(); x.CData("some <data> & stuff"); return x.Target() }, "<![CDATA[some <data> & stuff]]>"},

	{"cdata-split", func() string { x := New(); x.CData("a]]>b"); return x.Target() }, "<![CDATA[a]]]]><![CDATA[>b]]>"},

	{"comment", func() string { x := New(); x.Comment("a comment"); return x.Target() }, "<!-- a comment -->"},

	{"comment-noescape", func() string { x := New(); x.Comment("a -- b < c"); return x.Target() }, "<!-- a -- b < c -->"},

	{"instruct-default", func() string { x := New(); x.Instruct("", nil); return x.Target() }, `<?xml version="1.0" encoding="UTF-8"?>`},

	{"instruct-override", func() string {
		x := New()
		x.Instruct("xml", Of("version", "1.1", "encoding", "US-ASCII"))
		return x.Target()
	}, `<?xml version="1.1" encoding="US-ASCII"?>`},

	{"instruct-standalone", func() string {
		x := New()
		x.Instruct("xml", Of("standalone", "yes"))
		return x.Target()
	}, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`},

	{"instruct-extra", func() string {
		x := New()
		x.Instruct("xml", Of("standalone", "yes", "foo", "bar"))
		return x.Target()
	}, `<?xml version="1.0" encoding="UTF-8" standalone="yes" foo="bar"?>`},

	{"instruct-pi", func() string {
		x := New()
		x.Instruct("xml_stylesheet", Of("type", "text/xsl", "href", "style.xsl"))
		return x.Target()
	}, `<?xml_stylesheet type="text/xsl" href="style.xsl"?>`},

	{"declare", func() string { x := New(); x.Declare("DOCTYPE", Sym("html")); return x.Target() }, "<!DOCTYPE html>"},

	{"declare-public", func() string {
		x := New()
		x.Declare("DOCTYPE", Sym("html"), Sym("PUBLIC"),
			"-//W3C//DTD XHTML 1.0 Strict//EN", "http://x.dtd")
		return x.Target()
	}, `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://x.dtd">`},

	{"declare-toS", func() string { x := New(); x.Declare("FOO", 42); return x.Target() }, `<!FOO "42">`},

	// Indentation.
	{"indent-nested", func() string {
		x := New(WithIndent(2))
		x.Tag("person", func() {
			x.Tag("name", "Alice")
			x.Tag("age", func() { x.Tag("years", 30) })
		})
		return x.Target()
	}, "<person>\n  <name>Alice</name>\n  <age>\n    <years>30</years>\n  </age>\n</person>\n"},

	{"indent-mix", func() string {
		x := New(WithIndent(2))
		x.Tag("root", func() { x.Tag("a", Of("id", 1), "t"); x.Tag("b") })
		return x.Target()
	}, "<root>\n  <a id=\"1\">t</a>\n  <b/>\n</root>\n"},

	{"indent-margin", func() string {
		x := New(WithIndent(2), WithMargin(1))
		x.Tag("a", func() { x.Tag("b") })
		return x.Target()
	}, "  <a>\n    <b/>\n  </a>\n"},

	{"indent-instruct", func() string {
		x := New(WithIndent(2))
		x.Instruct("", nil)
		x.Tag("root", func() { x.Tag("a") })
		return x.Target()
	}, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root>\n  <a/>\n</root>\n"},

	{"indent-instruct-only", func() string {
		x := New(WithIndent(2))
		x.Instruct("", nil)
		return x.Target()
	}, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"},

	{"indent-comment", func() string {
		x := New(WithIndent(2))
		x.Tag("root", func() { x.Comment("c"); x.Tag("a") })
		return x.Target()
	}, "<root>\n  <!-- c -->\n  <a/>\n</root>\n"},

	{"indent-cdata", func() string {
		x := New(WithIndent(2))
		x.Tag("root", func() { x.CData("d") })
		return x.Target()
	}, "<root>\n  <![CDATA[d]]>\n</root>\n"},

	{"indent-mixed-text", func() string {
		x := New(WithIndent(2))
		x.Tag("root", func() { x.Text("free"); x.Tag("a") })
		return x.Target()
	}, "<root>\nfree  <a/>\n</root>\n"},

	{"declare-block", func() string {
		x := New(WithIndent(2))
		x.Declare("DOCTYPE", Sym("html"), func() { x.Declare("ELEMENT", Sym("br"), "EMPTY") })
		return x.Target()
	}, "<!DOCTYPE html [\n  <!ELEMENT br \"EMPTY\">\n]>\n"},

	// UTF-8 passes through; an invalid XML control char is replaced.
	{"utf8", func() string { x := New(); x.Tag("a", "café ☃"); return x.Target() }, "<a>café ☃</a>"},
	{"invalid-char", func() string { x := New(); x.Tag("a", "x\x00y"); return x.Target() }, "<a>x�y</a>"},
	{"invalid-attr-char", func() string {
		x := New()
		x.Tag("a", Of("k", "x\x01y"))
		return x.Target()
	}, "<a k=\"x�y\"/>"},
}

func TestGoldens(t *testing.T) {
	for _, g := range goldens {
		t.Run(g.name, func(t *testing.T) {
			if got := g.fn(); got != g.want {
				t.Errorf("\n got %q\nwant %q", got, g.want)
			}
		})
	}
}

// TestBlockWithXmlMarkupArg exercises the func(*XmlMarkup) block variant (the
// other cases mostly use func()), covering both block dispatch branches.
func TestBlockWithXmlMarkupArg(t *testing.T) {
	x := New()
	x.Block("p", nil, func(b *XmlMarkup) { b.Tag("br"); b.Text("HI") })
	if got, want := x.Target(), "<p><br/>HI</p>"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestBlockWithAttrs exercises Block's attrs branch.
func TestBlockWithAttrs(t *testing.T) {
	x := New()
	x.Block("p", Of("id", 1), func(b *XmlMarkup) { b.Text("hi") })
	if got, want := x.Target(), `<p id="1">hi</p>`; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// TestMixTextAndBlockPanics verifies the gem's ArgumentError is reproduced as a
// panic with the same message.
func TestMixTextAndBlockPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if msg, _ := r.(string); msg != "XmlMarkup cannot mix a text argument with a block" {
			t.Fatalf("panic = %v", r)
		}
	}()
	x := New()
	x.Tag("age", 30, func() {})
}

// TestWithTarget checks the external-sink option and that a nil sink is ignored.
func TestWithTarget(t *testing.T) {
	var sink strings.Builder
	x := New(WithTarget(&sink))
	x.Tag("a", "hi")
	if got, w := sink.String(), "<a>hi</a>"; got != w {
		t.Errorf("sink = %q want %q", got, w)
	}
	if x.Target() != "<a>hi</a>" {
		t.Errorf("Target() = %q", x.Target())
	}

	// A nil sink falls back to the internal buffer.
	y := New(WithTarget(nil))
	y.Tag("b")
	if got, w := y.Target(), "<b/>"; got != w {
		t.Errorf("nil-sink Target = %q want %q", got, w)
	}
}

// TestOfOddTrailingKey covers Of's odd-length branch (a trailing key with no
// value becomes an empty attribute).
func TestOfOddTrailingKey(t *testing.T) {
	x := New()
	x.Tag("a", Of("k"))
	if got, w := x.Target(), `<a k=""/>`; got != w {
		t.Errorf("got %q want %q", got, w)
	}
}

// TestAttrValueKind exercises the Attr / non-Attrs attribute path (a lone Attr)
// and a plain-string element name via valueKey's Symbol key.
func TestAttrValueKind(t *testing.T) {
	x := New()
	x.Tag("a", Attr{Key: "id", Value: 7})
	if got, w := x.Target(), `<a id="7"/>`; got != w {
		t.Errorf("got %q want %q", got, w)
	}
}

// TestOfSymbolKey covers Of with a Symbol key (valueKey's ok path).
func TestOfSymbolKey(t *testing.T) {
	a := Of(Symbol("id"), 1)
	if a[0].Key != "id" {
		t.Errorf("key = %q", a[0].Key)
	}
}

// TestNilIgnoredArg covers Tag's nil-argument branch (nil is dropped).
func TestNilIgnoredArg(t *testing.T) {
	x := New()
	x.Tag("a", nil, "hi")
	if got, w := x.Target(), "<a>hi</a>"; got != w {
		t.Errorf("got %q want %q", got, w)
	}
}
