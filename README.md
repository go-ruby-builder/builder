<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-builder/brand/main/social/go-ruby-builder-builder.png" alt="go-ruby-builder/builder" width="720"></p>

# builder — go-ruby-builder

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-builder.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [`builder`](https://github.com/rails/builder)
gem** — `Builder::XmlMarkup`, the programmatic XML/markup generator. It emits
markup **byte-for-byte identical** to builder 3.3.0 running on MRI (Ruby ≥ 4.0),
validated by a differential oracle against the gem — **without any Ruby runtime**.

It is a markup backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler) and
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine).

> **method_missing, without method_missing.** Ruby drives Builder through
> `method_missing`: `xml.person { xml.name("Alice") }` turns the *missing* method
> name into an element name. Go has no `method_missing`, so this package exposes
> the same emitter through an explicit, dynamic API — `Tag(name, …)` — that the
> host drives: the rbgo binding intercepts a Ruby `method_missing(name, *args,
> &block)` and calls `Tag(name, …)`, so element naming still comes straight from
> the missing Ruby method.

## Features

Faithful port of `Builder::XmlMarkup`, validated against the gem on every
supported platform:

- **Elements** — nested elements via blocks, attributes (insertion-ordered,
  attribute-escaped), text content (character-data-escaped), and self-closing
  empty tags (`<br/>`). An empty-string body emits `<a></a>`, no body emits
  `<a/>` — exactly as the gem distinguishes them.
- **`Tag` / `tag!`** — the general emitter, including namespace shorthand
  (`NS("ns", "name")` → `<ns:name>`) and the gem's *"cannot mix a text argument
  with a block"* rule (a panic mirroring MRI's `ArgumentError`).
- **`Text` / `text!`**, **`Append` / `<<`** (verbatim, unescaped),
  **`CData` / `cdata!`** (with `]]>` splitting), **`Comment` / `comment!`**,
  **`Instruct` / `instruct!`** (`<?xml version="1.0" encoding="UTF-8"?>`, ordered
  version/encoding/standalone), and **`Declare` / `declare!`** (DOCTYPE).
- **Indentation** (`WithIndent(2)`), a starting **margin** (`WithMargin`), and an
  external **target** sink (`WithTarget`).
- **MRI-exact escaping** — character data escapes only `& < >`; attribute values
  additionally escape `"` → `&quot;`, `\n` → `&#10;`, `\r` → `&#13;` (a literal
  tab and an apostrophe are left intact); invalid XML characters become U+FFFD.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three operating systems.

## Install

```sh
go get github.com/go-ruby-builder/builder
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-builder/builder"
)

func main() {
	x := builder.New(builder.WithIndent(2))
	x.Instruct("", nil) // <?xml version="1.0" encoding="UTF-8"?>
	x.Block("person", builder.Of("id", 1), func(x *builder.XmlMarkup) {
		x.Tag("name", "Alice & Bob")
		x.Tag("age", 30)
		x.Comment("a note")
		x.CData("raw <chars> & such")
	})
	fmt.Print(x.Target())
	// <?xml version="1.0" encoding="UTF-8"?>
	// <person id="1">
	//   <name>Alice &amp; Bob</name>
	//   <age>30</age>
	//   <!-- a note -->
	//   <![CDATA[raw <chars> & such]]>
	// </person>
}
```

## API

```go
func New(opts ...Option) *XmlMarkup
func WithIndent(spaces int) Option // Builder::XmlMarkup.new(indent: n)
func WithMargin(level int) Option  // margin: n
func WithTarget(sink *strings.Builder) Option // target:

// The dynamic element emitter method_missing maps onto (the rbgo binding calls
// Tag with the element name taken from the intercepted Ruby method symbol).
func (x *XmlMarkup) Tag(name string, args ...any) string

// Typed conveniences over Tag.
func (x *XmlMarkup) Element(name string, attrs Attrs, content Value) string
func (x *XmlMarkup) Block(name string, attrs Attrs, fn func(*XmlMarkup)) string
func (x *XmlMarkup) NS(ns, name string, args ...any) string // xml.tag!(:ns, :name, …)

func (x *XmlMarkup) Text(v Value) string    // text!  (escaped)
func (x *XmlMarkup) Append(v Value) string  // <<     (verbatim)
func (x *XmlMarkup) CData(text string) string
func (x *XmlMarkup) Comment(text string) string
func (x *XmlMarkup) Instruct(directive string, attrs Attrs) string
func (x *XmlMarkup) Declare(inst string, args ...any) string
func (x *XmlMarkup) Target() string // target!

type Value = any            // string / Symbol / bool / int / *big.Int / float / Raw / …
type Symbol string          // bare name; Sym("html") for declare! identifiers
type Raw string             // verbatim markup, bypasses escaping
type Attr struct{ Key string; Value Value }
type Attrs []Attr
func Of(kv ...any) Attrs     // Of("id", 1, "class", "vip")
```

### How element naming is exposed for rbgo

The gem's `xml.person(…)` reaches `method_missing(:person, …)`, which calls
`tag!(:person, …)`. The rbgo binding does the same across the language boundary:
it defines `XmlMarkup#method_missing` to forward `(name, *args, &block)` to this
package's `Tag(name.to_s, …)`, mapping each Ruby argument to a Go `Value` — a
`Hash` → `Attrs`, a block → `func(*XmlMarkup)`, everything else → text. So the
Go side never guesses element names; they arrive from Ruby, unchanged.

## Tests & coverage

The suite pairs deterministic, ruby-free **golden vectors** (which alone hold
coverage at 100%, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential oracle**: a wide corpus (nested elements, attributes, cdata,
comments, instruct, indentation, namespaces, escaping) is emitted here and by the
`builder` gem under the system `ruby`, and the bytes are required to match. The
oracle skips itself where `ruby`/the gem is absent or `RUBY_VERSION < "4.0"`.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-builder/builder authors.
