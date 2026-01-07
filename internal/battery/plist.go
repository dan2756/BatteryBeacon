package battery

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// decodePlist decodes an XML property list into Go values: map[string]any
// for <dict>, []any for <array>, int64, float64, string and bool. <data> is
// skipped and decodes as nil. An empty document decodes as nil.
func decodePlist(r io.Reader) (any, error) {
	d := xml.NewDecoder(r)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue // <?xml?>, <!DOCTYPE>, whitespace
		}
		if se.Name.Local == "plist" {
			continue
		}
		return decodePlistValue(d, se)
	}
}

func decodePlistValue(d *xml.Decoder, se xml.StartElement) (any, error) {
	switch se.Name.Local {
	case "dict":
		m := map[string]any{}
		var key *string
		for {
			tok, err := d.Token()
			if err != nil {
				return nil, err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "key" {
					var k string
					if err := d.DecodeElement(&k, &t); err != nil {
						return nil, err
					}
					key = &k
					continue
				}
				if key == nil {
					return nil, fmt.Errorf("plist: <%s> without a <key>", t.Name.Local)
				}
				v, err := decodePlistValue(d, t)
				if err != nil {
					return nil, err
				}
				m[*key] = v
				key = nil
			case xml.EndElement:
				return m, nil
			}
		}
	case "array":
		var a []any
		for {
			tok, err := d.Token()
			if err != nil {
				return nil, err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				v, err := decodePlistValue(d, t)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			case xml.EndElement:
				return a, nil
			}
		}
	case "true", "false":
		if err := d.Skip(); err != nil {
			return nil, err
		}
		return se.Name.Local == "true", nil
	case "data":
		return nil, d.Skip()
	}

	var s string
	if err := d.DecodeElement(&s, &se); err != nil {
		return nil, err
	}
	s = strings.TrimSpace(s)
	switch se.Name.Local {
	case "string", "date":
		return s, nil
	case "integer":
		return parsePlistInt(s)
	case "real":
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("plist: bad <real> %q", s)
		}
		return f, nil
	}
	return nil, fmt.Errorf("plist: unexpected <%s>", se.Name.Local)
}

// parsePlistInt parses an <integer>. ioreg prints signed registers such as
// Amperage as unsigned 64-bit numbers, so -1207 shows up as
// 18446744073709550409; those wrap back around to negative.
func parsePlistInt(s string) (int64, error) {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, nil
	}
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("plist: bad <integer> %q", s)
	}
	return int64(u), nil
}
