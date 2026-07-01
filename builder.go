// Copyright (c) the go-ruby-builder/builder authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package builder is a pure-Go (CGO=0), MRI-faithful reimplementation of Ruby's
// `builder` gem — Builder::XmlMarkup, the programmatic XML/markup generator.
//
// Ruby drives Builder through method_missing: `xml.person { xml.name("Alice") }`
// turns the missing method name into an element name. Go has no method_missing,
// so this package exposes the same emitter through an explicit, dynamic API that
// a host (go-embedded-ruby / rbgo) drives:
//
//   - [XmlMarkup.Tag] is the general element emitter — the Go equivalent of
//     Ruby's `xml.tag!`. The rbgo binding maps a Ruby `method_missing(name, …)`
//     straight onto Tag(name, …), so the element name comes from the missing
//     method just as in MRI.
//   - [XmlMarkup.Element] is a small typed convenience over Tag for the common
//     name+attrs+content shape.
//   - [XmlMarkup.Text], [XmlMarkup.CData], [XmlMarkup.Comment],
//     [XmlMarkup.Instruct], [XmlMarkup.Declare] and [XmlMarkup.Append] mirror
//     Builder's `text!`, `cdata!`, `comment!`, `instruct!`, `declare!` and `<<`.
//
// The bytes it emits are byte-for-byte identical to builder 3.3.0 running on
// MRI (Ruby >= 4.0), validated by a differential oracle against the gem.
package builder

import "strings"

// XmlMarkup is a Builder::XmlMarkup emitter. Create one with [New]; write to it
// with the element and special methods; read the accumulated document with
// [XmlMarkup.Target]. It is not safe for concurrent use.
type XmlMarkup struct {
	buf      *strings.Builder // internal accumulator when no external target
	target   *strings.Builder // where markup is appended (buf, or an aliased sink)
	indent   int              // spaces per level (0 = no indentation, all on one line)
	level    int              // current nesting depth (grows inside blocks)
	quote    byte             // attribute quote char (Ruby's @quote; default '"')
	encoding string           // declared encoding (instruct! updates it)
}

// Option configures a new [XmlMarkup].
type Option func(*XmlMarkup)

// WithIndent sets the number of spaces emitted per nesting level, matching
// Builder::XmlMarkup.new(indent: n). Zero (the default) emits everything on a
// single line with no newlines.
func WithIndent(spaces int) Option {
	return func(x *XmlMarkup) { x.indent = spaces }
}

// WithMargin sets the initial indentation level (Builder's margin: option), so
// the whole document is shifted right by margin*indent spaces. It has no effect
// unless an indent is also set.
func WithMargin(level int) Option {
	return func(x *XmlMarkup) { x.level = level }
}

// WithTarget directs output to an external *strings.Builder rather than the
// emitter's own buffer — the Go equivalent of Builder's target: option, which
// also lets one builder feed another. [XmlMarkup.Target] still returns the same
// accumulated text.
func WithTarget(sink *strings.Builder) Option {
	return func(x *XmlMarkup) { x.target = sink }
}

// New creates an XmlMarkup emitter. With no options it behaves like
// Builder::XmlMarkup.new: no indentation, double-quoted attributes, UTF-8.
func New(opts ...Option) *XmlMarkup {
	x := &XmlMarkup{
		buf:      &strings.Builder{},
		quote:    '"',
		encoding: "utf-8",
	}
	x.target = x.buf
	for _, opt := range opts {
		opt(x)
	}
	if x.target == nil {
		x.target = x.buf
	}
	return x
}

// Target returns the markup accumulated so far — Builder's target!.
func (x *XmlMarkup) Target() string { return x.target.String() }

// Attr is a single XML attribute. Builder takes attributes as a Ruby Hash, whose
// insertion order is preserved in the emitted markup; an ordered slice of Attr
// reproduces that exactly and avoids Go map's random iteration order.
type Attr struct {
	Key   string
	Value Value // rendered with Ruby to_s then attribute-escaped; nil -> ""
}

// Attrs is an ordered attribute list.
type Attrs []Attr

// Of builds an Attrs from alternating key/value pairs, a terse form for hosts
// and tests: Of("id", 1, "class", "vip"). An odd trailing key is given a nil
// value (rendered as an empty attribute), matching Builder's nil handling.
func Of(kv ...any) Attrs {
	a := make(Attrs, 0, (len(kv)+1)/2)
	for i := 0; i < len(kv); i += 2 {
		key := valueKey(kv[i])
		if i+1 < len(kv) {
			a = append(a, Attr{Key: key, Value: kv[i+1]})
		} else {
			a = append(a, Attr{Key: key, Value: nil})
		}
	}
	return a
}

// valueKey renders an attribute key (or element name fragment) to a string. A
// Symbol or a plain string is used verbatim; anything else falls back to to_s.
func valueKey(v any) string {
	if s, ok := valueToString(v); ok {
		return s
	}
	return ""
}

// Tag emits an element, the general form equivalent to Ruby's xml.tag!(name, …)
// (and to what method_missing dispatches to for xml.<name>(…)). Arguments after
// the name are interpreted like Builder does:
//
//   - an [Attrs] (or a lone [Attr]) supplies attributes; several are merged;
//   - a nil is ignored (Builder only records it under explicit-nil handling,
//     which this emitter, like the gem's default, does not enable);
//   - a func(*XmlMarkup) or func() is the nested block; and
//   - anything else is text content (multiple are concatenated).
//
// Mixing a text argument with a block panics with the same message MRI raises
// (ArgumentError: "XmlMarkup cannot mix a text argument with a block"). With
// neither text nor block the tag self-closes (<name/>); an empty string counts
// as text, so xml.Tag("a", "") emits <a></a>, matching the gem.
//
// The rbgo binding calls Tag with the element name taken from the intercepted
// Ruby method_missing symbol, so element naming comes straight from Ruby.
func (x *XmlMarkup) Tag(name string, args ...any) string {
	var (
		attrs    Attrs
		text     *string
		block    func(*XmlMarkup)
		hasBlock bool
	)
	for _, arg := range args {
		switch a := arg.(type) {
		case nil:
			// Ignored (default, non-explicit-nil handling).
		case Attrs:
			attrs = append(attrs, a...)
		case Attr:
			attrs = append(attrs, a)
		case func(*XmlMarkup):
			block, hasBlock = a, true
		case func():
			block, hasBlock = func(*XmlMarkup) { a() }, true
		default:
			s, _ := valueToString(a)
			if text == nil {
				text = &s
			} else {
				*text += s
			}
		}
	}

	if hasBlock {
		if text != nil {
			panic("XmlMarkup cannot mix a text argument with a block")
		}
		x.indentLine()
		x.startTag(name, attrs, false)
		x.newline()
		func() {
			defer func() {
				x.indentLine()
				x.endTag(name)
				x.newline()
			}()
			x.level++
			defer func() { x.level-- }()
			block(x)
		}()
		return x.Target()
	}
	if text == nil {
		x.indentLine()
		x.startTag(name, attrs, true)
		x.newline()
		return x.Target()
	}
	x.indentLine()
	x.startTag(name, attrs, false)
	x.Text(*text)
	x.endTag(name)
	x.newline()
	return x.Target()
}

// Element is a typed convenience over [Tag] for the frequent name+attrs+content
// shape. A nil content self-closes the tag; a non-nil content (including "") is
// emitted as escaped text.
func (x *XmlMarkup) Element(name string, attrs Attrs, content Value) string {
	if content == nil {
		if len(attrs) == 0 {
			return x.Tag(name)
		}
		return x.Tag(name, attrs)
	}
	if len(attrs) == 0 {
		return x.Tag(name, content)
	}
	return x.Tag(name, attrs, content)
}

// Block emits an element whose body is produced by fn — the block form,
// xml.name { … }. Attributes are optional.
func (x *XmlMarkup) Block(name string, attrs Attrs, fn func(*XmlMarkup)) string {
	if len(attrs) == 0 {
		return x.Tag(name, fn)
	}
	return x.Tag(name, attrs, fn)
}

// NS is Builder's namespace shorthand xml.tag!(:ns, :name, …): it joins the
// namespace prefix and local name with a colon, then behaves like [Tag].
func (x *XmlMarkup) NS(ns, name string, args ...any) string {
	return x.Tag(ns+":"+name, args...)
}

// Text appends escaped character data — Builder's text!. Unlike a string passed
// to Text, [Append] inserts markup verbatim.
func (x *XmlMarkup) Text(v Value) string {
	s, _ := valueToString(v)
	x.target.WriteString(escapeText(s))
	return x.Target()
}

// Append inserts markup verbatim, with no escaping — Builder's `xml << str`.
func (x *XmlMarkup) Append(v Value) string {
	s, _ := valueToString(v)
	x.target.WriteString(s)
	return x.Target()
}

// CData wraps text in a CDATA section — Builder's cdata!. A literal "]]>" in the
// text is split as "]]]]><![CDATA[>" so it cannot terminate the section early,
// exactly as the gem does. The text itself is not escaped.
func (x *XmlMarkup) CData(text string) string {
	safe := strings.ReplaceAll(text, "]]>", "]]]]><![CDATA[>")
	return x.special("<![CDATA[", "]]>", safe, nil)
}

// Comment emits an XML comment — Builder's comment!. The text is not escaped
// (the gem does not escape comment bodies).
func (x *XmlMarkup) Comment(text string) string {
	return x.special("<!-- ", " -->", text, nil)
}

// Instruct emits a processing instruction — Builder's instruct!. Called with no
// directive it emits the XML declaration <?xml version="1.0" encoding="UTF-8"?>;
// callers may override version/encoding/standalone (and add more attrs) via
// opts. A non-"xml" directive emits <?directive …?> with the given attributes.
func (x *XmlMarkup) Instruct(directive string, attrs Attrs) string {
	if directive == "" {
		directive = "xml"
	}
	if directive == "xml" {
		attrs = mergeXMLDeclDefaults(attrs)
		if enc, ok := attrs.get("encoding"); ok {
			if s, ok := valueToString(enc); ok {
				x.encoding = strings.ToLower(s)
			}
		}
	}
	return x.special("<?"+directive, "?>", "", orderedForInstruct(attrs))
}

// Declare emits a markup declaration such as a DOCTYPE — Builder's declare!.
// Each arg is emitted per its type: a [Symbol] (an unquoted identifier, e.g.
// Sym("html")) prints bare, a string prints double-quoted, and a func (func()
// or func(*XmlMarkup)) is the internal-subset block, emitted between " [" and
// "]" with its body nested one level deeper. Other values print double-quoted
// via to_s.
func (x *XmlMarkup) Declare(inst string, args ...any) string {
	x.indentLine()
	x.target.WriteString("<!" + inst)
	var block func(*XmlMarkup)
	for _, arg := range args {
		switch a := arg.(type) {
		case Symbol:
			x.target.WriteString(" " + string(a))
		case string:
			x.target.WriteString(` "` + a + `"`)
		case func(*XmlMarkup):
			block = a
		case func():
			block = func(*XmlMarkup) { a() }
		default:
			s, _ := valueToString(a)
			x.target.WriteString(` "` + s + `"`)
		}
	}
	if block != nil {
		x.target.WriteString(" [")
		x.newline()
		x.level++
		block(x)
		x.level--
		x.target.WriteString("]")
	}
	x.target.WriteString(">")
	x.newline()
	return x.Target()
}

// Sym marks a declaration argument as an unquoted identifier (a Ruby Symbol), as
// in xml.declare!(:DOCTYPE, :html). It is just [Symbol] with a shorter name at
// the declaration call site.
func Sym(name string) Symbol { return Symbol(name) }

// special implements Builder's _special: optional indent, an open marker, an
// optional data payload, optional ordered attributes, a close marker, newline.
func (x *XmlMarkup) special(open, close, data string, attrs Attrs) string {
	x.indentLine()
	x.target.WriteString(open)
	x.target.WriteString(data)
	x.insertAttributes(attrs)
	x.target.WriteString(close)
	x.newline()
	return x.Target()
}

// startTag writes "<name attrs>" (or "<name attrs/>" when selfClose).
func (x *XmlMarkup) startTag(name string, attrs Attrs, selfClose bool) {
	x.target.WriteString("<" + name)
	x.insertAttributes(attrs)
	if selfClose {
		x.target.WriteString("/")
	}
	x.target.WriteString(">")
}

// endTag writes "</name>".
func (x *XmlMarkup) endTag(name string) { x.target.WriteString("</" + name + ">") }

// insertAttributes writes each attribute as ` key="value"` in order, with the
// value attribute-escaped. Matches Builder's _insert_attributes for an
// insertion-ordered Hash (a nil value renders as an empty string).
func (x *XmlMarkup) insertAttributes(attrs Attrs) {
	for _, a := range attrs {
		s, _ := valueToString(a.Value)
		x.target.WriteByte(' ')
		x.target.WriteString(a.Key)
		x.target.WriteByte('=')
		x.target.WriteByte(x.quote)
		x.target.WriteString(escapeAttr(s))
		x.target.WriteByte(x.quote)
	}
}

// indentLine writes the current level's indentation — Builder's _indent (a
// no-op when indent is 0 or at the top level).
func (x *XmlMarkup) indentLine() {
	if x.indent == 0 || x.level == 0 {
		return
	}
	x.target.WriteString(strings.Repeat(" ", x.level*x.indent))
}

// newline writes a newline — Builder's _newline (a no-op when indent is 0).
func (x *XmlMarkup) newline() {
	if x.indent == 0 {
		return
	}
	x.target.WriteString("\n")
}

// get returns the first attribute value for key, if present.
func (a Attrs) get(key string) (Value, bool) {
	for _, at := range a {
		if at.Key == key {
			return at.Value, true
		}
	}
	return nil, false
}

// mergeXMLDeclDefaults returns version/encoding defaults merged under the
// caller's overrides, as Builder does for instruct! :xml. Caller-supplied keys
// win and keep their position; missing version/encoding are prepended in order.
func mergeXMLDeclDefaults(attrs Attrs) Attrs {
	out := Attrs{{Key: "version", Value: "1.0"}, {Key: "encoding", Value: "UTF-8"}}
	for _, a := range attrs {
		if i := out.index(a.Key); i >= 0 {
			out[i].Value = a.Value
		} else {
			out = append(out, a)
		}
	}
	return out
}

// index returns the position of key in a, or -1.
func (a Attrs) index(key string) int {
	for i, at := range a {
		if at.Key == key {
			return i
		}
	}
	return -1
}

// orderedForInstruct reorders attributes into Builder's instruct! order —
// version, encoding, standalone first (when present), then the rest in their
// original order — matching _special's order argument.
func orderedForInstruct(attrs Attrs) Attrs {
	order := []string{"version", "encoding", "standalone"}
	out := make(Attrs, 0, len(attrs))
	used := make(map[int]bool, len(attrs))
	for _, key := range order {
		for i, a := range attrs {
			if !used[i] && a.Key == key {
				out = append(out, a)
				used[i] = true
			}
		}
	}
	for i, a := range attrs {
		if !used[i] {
			out = append(out, a)
		}
	}
	return out
}
