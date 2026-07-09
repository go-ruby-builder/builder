# frozen_string_literal: true

# Builder::XmlMarkup — programmatic XML/markup generation.
# A missing method name becomes an element; a Hash becomes attributes; a block
# nests children. text! escapes character data, cdata! wraps a CDATA section,
# comment! emits a comment, and target! returns the accumulated markup.
require "builder"

xml = Builder::XmlMarkup.new(indent: 2)
xml.instruct!                       # <?xml version="1.0" encoding="UTF-8"?>
xml.catalog do
  xml.book(id: "b1", lang: "en") do # attributes render in insertion order
    xml.title("Ruby")
    xml.author("Matz")
    xml.note { xml.text!("Tom & Jerry < Co") } # & and < are escaped
  end
  xml.comment!("end of catalog")
  xml.data { xml.cdata!("<raw/>") } # verbatim, wrapped in <![CDATA[ ... ]]>
end

puts xml.target!                    # the generated document
