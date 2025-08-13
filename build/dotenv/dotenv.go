package dotenv

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// errUnterminatedQuote is returned when a quoted value is not properly terminated.
var errUnterminatedQuote = errors.New("unterminated quoted value")

// Parse parses a .env formatted string into key/value pairs.
//
// Supports lines like:
//
//	KEY=value
//	KEY="quoted # not comment"
//	KEY='single quoted'
//	export KEY=value
//
// Comments start with '#' when unquoted (at start or preceded by whitespace).
func Parse(data string) (map[string]string, error) {
	res := make(map[string]string)

	// Normalize line endings.
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")

	lines := strings.Split(data, "\n")
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// Strip BOM on first line if present.
		if i == 0 && strings.HasPrefix(line, "\uFEFF") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "\uFEFF"))
		}
		// Full-line comment.
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Optional export prefix (allow any whitespace after 'export').
		if strings.HasPrefix(line, "export") {
			after := line[len("export"):]
			if len(after) > 0 && isSpace(after[0]) {
				line = strings.TrimSpace(after)
			}
		}

		eq := strings.IndexByte(line, '=')
		if eq == -1 {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed .env line %d (no '=')\n", i+1)
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed .env line %d (empty key)\n", i+1)
			continue
		}
		valuePart := strings.TrimSpace(line[eq+1:])

		var (
			val string
			err error
		)

		// If the value starts quoted and doesn't close on the same line,
		// keep appending subsequent lines until we find the closing quote.
		if startsWithQuote(valuePart) {
			var consumed int
			val, consumed, err = collectQuotedValue(valuePart, lines, i)
			if err != nil {
				return nil, err
			}
			// Skip the extra lines we consumed for this value.
			i += consumed
		} else {
			val, err = parseEnvValue(valuePart)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
		}

		res[key] = val
	}
	return res, nil
}

// parseEnvValue parses a single .env value (possibly quoted) and strips trailing comments when unquoted.
func parseEnvValue(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if strings.HasPrefix(s, "\"") {
		v, err := parseQuotedValue(s, '"', true)
		return v, err
	}
	if strings.HasPrefix(s, "'") {
		v, err := parseQuotedValue(s, '\'', false)
		return v, err
	}
	// Unquoted value: stop before an unquoted comment start '#'
	// when it's the first character or preceded by whitespace.
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || isSpace(s[i-1])) {
			return strings.TrimSpace(s[:i]), nil
		}
	}
	return strings.TrimSpace(s), nil
}

// startsWithQuote reports whether the value begins with a single or double quote.
func startsWithQuote(s string) bool {
	return len(s) > 0 && (s[0] == '"' || s[0] == '\'')
}

// collectQuotedValue accumulates a possibly multi-line quoted value until the closing quote.
// Returns the parsed value, the number of extra lines consumed (beyond the current one), or an error.
func collectQuotedValue(valuePart string, lines []string, startIdx int) (string, int, error) {
	delim := valuePart[0]
	unescape := delim == '"'
	combined := valuePart
	consumed := 0

	for {
		v, err := parseQuotedValue(combined, delim, unescape)
		switch {
		case err == nil:
			return v, consumed, nil
		case errors.Is(err, errUnterminatedQuote):
			// Need another line. If none left, report a wrapped unterminated-quote error with line context.
			if startIdx+consumed+1 >= len(lines) {
				return "", 0, fmt.Errorf("line %d: %w", startIdx+1, errUnterminatedQuote)
			}
			consumed++
			combined += "\n" + lines[startIdx+consumed]
		default:
			return "", 0, fmt.Errorf("line %d: %w", startIdx+1, err)
		}
	}
}

// parseQuotedValue parses a value starting with the given delimiter.
// If unescape is true, common backslash escapes (\n,\r,\t,\",\\) are processed.
func parseQuotedValue(s string, delim byte, unescape bool) (val string, err error) {
	var b strings.Builder
	escaped := false
	for i := 1; i < len(s); i++ {
		ch := s[i]
		if escaped {
			if unescape {
				switch ch {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				case '\\':
					b.WriteByte('\\')
				case '"':
					b.WriteByte('"')
				default:
					// Preserve the backslash for unknown escapes.
					b.WriteByte('\\')
					b.WriteByte(ch)
				}
			} else {
				// In single-quoted values, keep escapes literally.
				b.WriteByte('\\')
				b.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' && unescape {
			escaped = true
			continue
		}
		if ch == delim {
			// Ignore anything after the closing quote (comments, etc).
			return b.String(), nil
		}
		b.WriteByte(ch)
	}
	return "", errUnterminatedQuote
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}
