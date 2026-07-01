package ldif

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const maxLineLength = 76

// Format returns LDIF text for records.
func Format(records []Record) string {
	var b strings.Builder
	for i, record := range records {
		if i > 0 {
			b.WriteByte('\n')
		}
		writeAttributeLine(&b, "dn", record.DN)
		for _, attr := range record.Attributes {
			writeAttributeLine(&b, attr.Name, attr.Value)
		}
	}
	return b.String()
}

// Write writes LDIF text for records.
func Write(w io.Writer, records []Record) error {
	_, err := io.WriteString(w, Format(records))
	return err
}

func writeAttributeLine(b *strings.Builder, name, value string) {
	line := formatAttributeLine(name, value)
	writeFoldedLine(b, line)
}

func formatAttributeLine(name, value string) string {
	if requiresBase64(value) {
		return fmt.Sprintf("%s:: %s", name, base64.StdEncoding.EncodeToString([]byte(value)))
	}
	if value == "" {
		return fmt.Sprintf("%s:", name)
	}
	return fmt.Sprintf("%s: %s", name, value)
}

func requiresBase64(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, " ") || strings.HasPrefix(value, ":") || strings.HasPrefix(value, "<") {
		return true
	}
	if strings.HasSuffix(value, " ") {
		return true
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == '\n' || c == '\r' || c == 0 || c < 0x20 || c >= 0x7f {
			return true
		}
	}
	return false
}

func writeFoldedLine(b *strings.Builder, line string) {
	for len(line) > maxLineLength {
		b.WriteString(line[:maxLineLength])
		b.WriteByte('\n')
		line = " " + line[maxLineLength:]
	}
	b.WriteString(line)
	b.WriteByte('\n')
}
