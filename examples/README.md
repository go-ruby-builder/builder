# builder examples

Runnable, pure-Ruby usage of `Builder::XmlMarkup`, verified under the
[rbgo](https://github.com/go-embedded-ruby/ruby) interpreter.

```sh
rbgo examples/builder_usage.rb
```

| What it shows | API |
| --- | --- |
| XML declaration | `instruct!` |
| Elements from missing method names | `xml.catalog { ... }` |
| Attributes (insertion order) | `xml.book(id:, lang:)` |
| Escaped character data | `text!` |
| CDATA section | `cdata!` |
| XML comment | `comment!` |
| Indentation and output | `new(indent: 2)`, `target!` |
