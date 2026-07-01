// Copyright (c) the go-ruby-builder/builder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"os/exec"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` that is at least 4.0 and can load the builder
// gem. The oracle tests skip themselves otherwise (the qemu cross-arch and
// Windows lanes, or where the gem is absent), so the deterministic golden suite
// alone drives the 100% coverage gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	// Version gate: RUBY_VERSION >= "4.0".
	out, err := exec.Command(path, "-e", "print RUBY_VERSION").CombinedOutput()
	if err != nil {
		t.Skipf("cannot query ruby version: %v", err)
	}
	if !rubyAtLeast4(string(out)) {
		t.Skipf("ruby %s < 4.0; skipping MRI oracle", strings.TrimSpace(string(out)))
	}
	// The builder gem must be loadable.
	if err := exec.Command(path, "-e", "require 'builder'").Run(); err != nil {
		t.Skip("builder gem not installed; skipping MRI oracle")
	}
	return path
}

// rubyAtLeast4 reports whether the dotted version string is >= 4.0.
func rubyAtLeast4(v string) bool {
	major, _, _ := strings.Cut(strings.TrimSpace(v), ".")
	n := 0
	for _, r := range major {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 4
}

// rubyBuilder runs a builder program in MRI and returns x.target!. The script
// body builds into an XmlMarkup named `x`; the preamble binds $stdout and the
// gem so Windows text-mode never pollutes the bytes (the go-ruby-erb lesson).
func rubyBuilder(t *testing.T, bin, ctor, body string) string {
	t.Helper()
	script := "$stdout.binmode\nrequire 'builder'\n" +
		"x = " + ctor + "\n" + body + "\nprint x.target!\n"
	out, err := exec.Command(bin, "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// oracleCase pairs a Go builder program with the equivalent Ruby builder program
// so the emitted bytes can be required to match exactly.
type oracleCase struct {
	name string
	ctor string        // Ruby constructor, e.g. `Builder::XmlMarkup.new(indent: 2)`
	body string        // Ruby body building into `x`
	go_  func() string // the equivalent Go program
}

// oracleCases is the differential corpus: nested elements, attributes, cdata,
// comments, instruct, indentation, namespaces, and escaping — the surface the
// task calls out. Each Go program must emit byte-for-byte what MRI's builder does.
var oracleCases = []oracleCase{
	{
		name: "nested",
		ctor: "Builder::XmlMarkup.new",
		body: `x.person { x.name("Alice"); x.age(30) }`,
		go_: func() string {
			x := New()
			x.Tag("person", func() { x.Tag("name", "Alice"); x.Tag("age", 30) })
			return x.Target()
		},
	},
	{
		name: "attrs",
		ctor: "Builder::XmlMarkup.new",
		body: `x.person(:id => 1, "class" => "vip") { x.name("Bob") }`,
		go_: func() string {
			x := New()
			x.Tag("person", Of("id", 1, "class", "vip"), func() { x.Tag("name", "Bob") })
			return x.Target()
		},
	},
	{
		name: "escaping",
		ctor: "Builder::XmlMarkup.new",
		body: `x.p("a < b & c > d \" ' ", :t => "x<y&z>\"'")`,
		go_: func() string {
			x := New()
			x.Tag("p", Of("t", `x<y&z>"'`), "a < b & c > d \" ' ")
			return x.Target()
		},
	},
	{
		name: "attr-whitespace",
		ctor: "Builder::XmlMarkup.new",
		body: `x.a(:t => "x\ny\tz\rw")`,
		go_: func() string {
			x := New()
			x.Tag("a", Of("t", "x\ny\tz\rw"))
			return x.Target()
		},
	},
	{
		name: "empty-and-self-close",
		ctor: "Builder::XmlMarkup.new",
		body: `x.br; x.img(:src => "a.png"); x.a("")`,
		go_: func() string {
			x := New()
			x.Tag("br")
			x.Tag("img", Of("src", "a.png"))
			x.Tag("a", "")
			return x.Target()
		},
	},
	{
		name: "namespace",
		ctor: "Builder::XmlMarkup.new",
		body: `x.tag!("soap:Envelope", :xmlns => "http://x") { x.tag!("soap:Body") }`,
		go_: func() string {
			x := New()
			x.NS("soap", "Envelope", Of("xmlns", "http://x"), func() { x.Tag("soap:Body") })
			return x.Target()
		},
	},
	{
		name: "cdata-and-comment",
		ctor: "Builder::XmlMarkup.new",
		body: `x.root { x.comment!("note"); x.cdata!("some <d> & a]]>b") }`,
		go_: func() string {
			x := New()
			x.Tag("root", func() { x.Comment("note"); x.CData("some <d> & a]]>b") })
			return x.Target()
		},
	},
	{
		name: "instruct",
		ctor: "Builder::XmlMarkup.new",
		body: `x.instruct!; x.instruct!(:xml, :standalone => "yes")`,
		go_: func() string {
			x := New()
			x.Instruct("", nil)
			x.Instruct("xml", Of("standalone", "yes"))
			return x.Target()
		},
	},
	{
		name: "instruct-pi",
		ctor: "Builder::XmlMarkup.new",
		body: `x.instruct!(:xml_stylesheet, :type => "text/xsl", :href => "s.xsl")`,
		go_: func() string {
			x := New()
			x.Instruct("xml_stylesheet", Of("type", "text/xsl", "href", "s.xsl"))
			return x.Target()
		},
	},
	{
		name: "declare",
		ctor: "Builder::XmlMarkup.new",
		body: `x.declare!(:DOCTYPE, :html, :PUBLIC, "-//W3C//DTD XHTML 1.0 Strict//EN", "http://x.dtd")`,
		go_: func() string {
			x := New()
			x.Declare("DOCTYPE", Sym("html"), Sym("PUBLIC"),
				"-//W3C//DTD XHTML 1.0 Strict//EN", "http://x.dtd")
			return x.Target()
		},
	},
	{
		name: "indent",
		ctor: "Builder::XmlMarkup.new(:indent => 2)",
		body: `x.instruct!; x.person(:id => 1) { x.name("Alice"); x.age { x.years(30) }; x.comment!("c"); x.cdata!("d") }`,
		go_: func() string {
			x := New(WithIndent(2))
			x.Instruct("", nil)
			x.Tag("person", Of("id", 1), func() {
				x.Tag("name", "Alice")
				x.Tag("age", func() { x.Tag("years", 30) })
				x.Comment("c")
				x.CData("d")
			})
			return x.Target()
		},
	},
	{
		name: "indent-margin",
		ctor: "Builder::XmlMarkup.new(:indent => 2, :margin => 1)",
		body: `x.a { x.b { x.c("d") } }`,
		go_: func() string {
			x := New(WithIndent(2), WithMargin(1))
			x.Tag("a", func() { x.Tag("b", func() { x.Tag("c", "d") }) })
			return x.Target()
		},
	},
	{
		name: "declare-block",
		ctor: "Builder::XmlMarkup.new(:indent => 2)",
		body: `x.declare!(:DOCTYPE, :html) { x.declare!(:ELEMENT, :br, "EMPTY") }`,
		go_: func() string {
			x := New(WithIndent(2))
			x.Declare("DOCTYPE", Sym("html"), func() { x.Declare("ELEMENT", Sym("br"), "EMPTY") })
			return x.Target()
		},
	},
	{
		name: "unicode",
		ctor: "Builder::XmlMarkup.new",
		body: `x.a("café ☃ €")`,
		go_: func() string {
			x := New()
			x.Tag("a", "café ☃ €")
			return x.Target()
		},
	},
	{
		name: "numeric-content",
		ctor: "Builder::XmlMarkup.new",
		body: `x.a(42); x.b(3.14); x.c(2.0)`,
		go_: func() string {
			x := New()
			x.Tag("a", 42)
			x.Tag("b", 3.14)
			x.Tag("c", 2.0)
			return x.Target()
		},
	},
}

// TestOracleDifferential requires this package's emitter to produce, for every
// corpus case, the exact bytes MRI's builder gem does.
func TestOracleDifferential(t *testing.T) {
	bin := rubyBin(t)
	for _, c := range oracleCases {
		t.Run(c.name, func(t *testing.T) {
			want := rubyBuilder(t, bin, c.ctor, c.body)
			if got := c.go_(); got != want {
				t.Errorf("emitter differs from MRI builder\n go:  %q\n mri: %q", got, want)
			}
		})
	}
}
