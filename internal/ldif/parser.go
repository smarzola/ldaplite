package ldif

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Attribute is one LDIF attribute value in record order.
type Attribute struct {
	Name  string
	Value string
	Line  int
}

// Record is one LDIF entry record.
type Record struct {
	DN         string
	DNLine     int
	Attributes []Attribute
}

// Values returns all values for name in record order.
func (r Record) Values(name string) []string {
	var values []string
	for _, attr := range r.Attributes {
		if strings.EqualFold(attr.Name, name) {
			values = append(values, attr.Value)
		}
	}
	if values == nil {
		return []string{}
	}
	return values
}

// FirstValue returns the first value for name.
func (r Record) FirstValue(name string) string {
	values := r.Values(name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// ParseError reports an LDIF parse error with source context.
type ParseError struct {
	Line int
	DN   string
	Msg  string
}

func (e *ParseError) Error() string {
	if e.DN != "" {
		return fmt.Sprintf("ldif line %d (%s): %s", e.Line, e.DN, e.Msg)
	}
	return fmt.Sprintf("ldif line %d: %s", e.Line, e.Msg)
}

// Parse parses LDIF entry records.
func Parse(input string) ([]Record, error) {
	lines, err := unfoldLines(input)
	if err != nil {
		return nil, err
	}

	var records []Record
	var current *Record
	for _, line := range lines {
		text := line.text
		if strings.TrimSpace(text) == "" {
			if current != nil {
				if err := finishRecord(current); err != nil {
					return nil, err
				}
				records = append(records, *current)
				current = nil
			}
			continue
		}
		if strings.HasPrefix(text, "#") {
			continue
		}
		if current == nil {
			current = &Record{}
		}
		if err := parseLine(current, text, line.number); err != nil {
			return nil, err
		}
	}

	if current != nil {
		if err := finishRecord(current); err != nil {
			return nil, err
		}
		records = append(records, *current)
	}

	return records, nil
}

type logicalLine struct {
	text   string
	number int
}

func unfoldLines(input string) ([]logicalLine, error) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	raw := strings.Split(input, "\n")

	lines := make([]logicalLine, 0, len(raw))
	for i, text := range raw {
		lineNumber := i + 1
		if strings.HasPrefix(text, " ") {
			if len(lines) == 0 {
				return nil, &ParseError{Line: lineNumber, Msg: "folded line has no previous line"}
			}
			lines[len(lines)-1].text += strings.TrimPrefix(text, " ")
			continue
		}
		lines = append(lines, logicalLine{text: text, number: lineNumber})
	}
	return lines, nil
}

func parseLine(record *Record, line string, lineNumber int) error {
	name, value, err := splitAttributeLine(line)
	if err != nil {
		return &ParseError{Line: lineNumber, DN: record.DN, Msg: err.Error()}
	}

	switch {
	case strings.EqualFold(name, "dn"):
		if record.DN != "" {
			return &ParseError{Line: lineNumber, DN: record.DN, Msg: "duplicate dn"}
		}
		record.DN = value
		record.DNLine = lineNumber
	case strings.EqualFold(name, "changetype"):
		return &ParseError{Line: lineNumber, DN: record.DN, Msg: "changetype records are not supported"}
	default:
		record.Attributes = append(record.Attributes, Attribute{Name: name, Value: value, Line: lineNumber})
	}
	return nil
}

func splitAttributeLine(line string) (string, string, error) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return "", "", fmt.Errorf("attribute line is missing ':'")
	}

	name := strings.TrimSpace(line[:idx])
	if name == "" {
		return "", "", fmt.Errorf("attribute name is required")
	}
	if strings.ContainsAny(name, " \t") {
		return "", "", fmt.Errorf("attribute name %q contains whitespace", name)
	}

	rest := line[idx+1:]
	switch {
	case strings.HasPrefix(rest, ":"):
		raw := strings.TrimSpace(rest[1:])
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return "", "", fmt.Errorf("invalid base64 value for %s: %w", name, err)
		}
		return name, string(decoded), nil
	case strings.HasPrefix(rest, "<"):
		return "", "", fmt.Errorf("URL values are not supported")
	default:
		return name, strings.TrimPrefix(rest, " "), nil
	}
}

func finishRecord(record *Record) error {
	if record.DN == "" {
		line := 1
		if len(record.Attributes) > 0 {
			line = record.Attributes[0].Line
		}
		return &ParseError{Line: line, Msg: "record is missing dn"}
	}
	return nil
}
