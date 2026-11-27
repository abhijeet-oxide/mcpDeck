package tool

import (
	"strings"

	"golang.org/x/net/html"
)

// Form is one HTML form (TOOL-12).
type Form struct {
	Name    string            `json:"name"`
	ID      string            `json:"id"`
	Action  string            `json:"action"`
	Method  string            `json:"method"`
	Enctype string            `json:"enctype"`
	Fields  []FormField       `json:"fields"`
	Values  map[string]any    `json:"values"` // name → value (or list, for repeated names), what a browser would post
	Hidden  map[string]string `json:"hidden"` // hidden fields only
}

// FormField is one input, select, textarea or button.
type FormField struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Value   string   `json:"value"`
	Label   string   `json:"label,omitempty"`
	Options []string `json:"options,omitempty"`
	Checked bool     `json:"checked,omitempty"`
}

func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, k) {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, k string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, k) {
			return true
		}
	}
	return false
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// ParseForms returns the forms in a page with every field, hidden and
// repeated ones included, and the values a browser would post.
func ParseForms(page string) []Form {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return []Form{}
	}
	out := []Form{}
	var walk func(n *html.Node, f *Form, label string)
	walk = func(n *html.Node, f *Form, label string) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "form":
				nf := Form{Name: attr(n, "name"), ID: attr(n, "id"), Action: attr(n, "action"), Method: strings.ToUpper(attr(n, "method")),
					Enctype: attr(n, "enctype"), Fields: []FormField{}, Values: map[string]any{}, Hidden: map[string]string{}}
				if nf.Method == "" {
					nf.Method = "GET"
				}
				if nf.Enctype == "" {
					nf.Enctype = "application/x-www-form-urlencoded"
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c, &nf, "")
				}
				out = append(out, nf)
				return
			case "label":
				label = textOf(n)
			case "input", "select", "textarea", "button":
				if f != nil && attr(n, "name") != "" {
					fld := FormField{Name: attr(n, "name"), Type: strings.ToLower(attr(n, "type")), Value: attr(n, "value"), Label: label}
					switch n.Data {
					case "select":
						fld.Type = "select"
						first := ""
						for o := range n.Descendants() {
							if o.Type == html.ElementNode && o.Data == "option" {
								v := attr(o, "value")
								if !hasAttr(o, "value") {
									v = textOf(o)
								}
								fld.Options = append(fld.Options, v)
								if first == "" {
									first = v
								}
								if hasAttr(o, "selected") {
									fld.Value = v
								}
							}
						}
						if fld.Value == "" {
							fld.Value = first
						}
					case "textarea":
						fld.Type, fld.Value = "textarea", textOf(n)
					case "button":
						if fld.Type == "" {
							fld.Type = "submit"
						}
					}
					if fld.Type == "" {
						fld.Type = "text"
					}
					fld.Checked = hasAttr(n, "checked")
					f.Fields = append(f.Fields, fld)
					posts := fld.Type != "submit" && fld.Type != "button" && fld.Type != "file" && fld.Type != "reset" && fld.Type != "image" &&
						((fld.Type != "checkbox" && fld.Type != "radio") || fld.Checked)
					if posts {
						switch prev := f.Values[fld.Name].(type) {
						case nil:
							f.Values[fld.Name] = fld.Value
						case string:
							f.Values[fld.Name] = []any{prev, fld.Value}
						case []any:
							f.Values[fld.Name] = append(prev, fld.Value)
						}
					}
					if fld.Type == "hidden" {
						f.Hidden[fld.Name] = fld.Value
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, f, label)
		}
	}
	walk(doc, nil, "")
	return out
}

// ParseTables returns each table's rows as objects keyed by its header
// cells (th, or the first row), plus the links in each row.
func ParseTables(page string) [][]map[string]string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return [][]map[string]string{}
	}
	out := [][]map[string]string{}
	for t := range doc.Descendants() {
		if t.Type != html.ElementNode || t.Data != "table" {
			continue
		}
		var header []string
		rows := []map[string]string{}
		for tr := range t.Descendants() {
			if tr.Type != html.ElementNode || tr.Data != "tr" {
				continue
			}
			var cells []string
			var hrefs []string
			th := false
			for c := tr.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					th = th || c.Data == "th"
					cells = append(cells, textOf(c))
					for a := range c.Descendants() {
						if a.Type == html.ElementNode && a.Data == "a" && attr(a, "href") != "" {
							hrefs = append(hrefs, attr(a, "href"))
						}
					}
				}
			}
			if len(cells) == 0 {
				continue
			}
			if header == nil {
				header = cells
				if th {
					continue
				}
				header = make([]string, len(cells))
				for i := range cells {
					header[i] = "col" + string(rune('1'+i))
				}
			}
			row := map[string]string{}
			for i, v := range cells {
				k := "col" + string(rune('1'+i))
				if i < len(header) && header[i] != "" {
					k = header[i]
				}
				row[k] = v
			}
			if len(hrefs) > 0 {
				row["href"] = hrefs[0]
			}
			rows = append(rows, row)
		}
		out = append(out, rows)
	}
	return out
}

// PageText is the visible text of a page, for finding a status or message.
func PageText(page string) string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return ""
	}
	return textOf(doc)
}

// TableHeaders lists each table's column headers: its th cells, or its
// first row's cells.
func TableHeaders(page string) [][]string {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return [][]string{}
	}
	out := [][]string{}
	for t := range doc.Descendants() {
		if t.Type != html.ElementNode || t.Data != "table" {
			continue
		}
		for tr := range t.Descendants() {
			if tr.Type != html.ElementNode || tr.Data != "tr" {
				continue
			}
			var cells []string
			for c := tr.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, textOf(c))
				}
			}
			if len(cells) > 0 {
				out = append(out, cells)
				break
			}
		}
	}
	return out
}
