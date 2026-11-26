package tool

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// ParseXML turns XML into plain values for tools: an element with only
// text becomes its text; otherwise an object whose attributes are "@name",
// text is "#text", and repeated children become arrays. Namespace
// prefixes are dropped, so soap:Envelope is Envelope.
func ParseXML(text string) (any, error) {
	dec := xml.NewDecoder(strings.NewReader(text))
	dec.Strict = false
	type frame struct {
		name string
		obj  map[string]any
		text strings.Builder
	}
	var stack []*frame
	root := map[string]any{}
	add := func(parent map[string]any, name string, v any) {
		if old, ok := parent[name]; ok {
			if list, ok := old.([]any); ok {
				parent[name] = append(list, v)
			} else {
				parent[name] = []any{old, v}
			}
			return
		}
		parent[name] = v
	}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			f := &frame{name: t.Name.Local, obj: map[string]any{}}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
					continue
				}
				f.obj["@"+a.Name.Local] = a.Value
			}
			stack = append(stack, f)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("unbalanced XML")
			}
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			txt := strings.TrimSpace(f.text.String())
			var v any = f.obj
			if len(f.obj) == 0 {
				v = txt
			} else if txt != "" {
				f.obj["#text"] = txt
			}
			if len(stack) == 0 {
				add(root, f.name, v)
			} else {
				add(stack[len(stack)-1].obj, f.name, v)
			}
		}
	}
	if len(root) == 0 {
		return nil, errors.New("no XML elements found")
	}
	return root, nil
}
