package issue

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var umlautMap = map[rune]string{
	'ä': "ae", 'ö': "oe", 'ü': "ue", 'ß': "ss",
	'Ä': "ae", 'Ö': "oe", 'Ü': "ue",
}

var multiDash = regexp.MustCompile(`-{2,}`)

func GenerateSlug(title string) string {
	s := strings.ToLower(title)

	// Replace umlauts
	var buf strings.Builder
	for _, r := range s {
		if repl, ok := umlautMap[r]; ok {
			buf.WriteString(repl)
		} else {
			buf.WriteRune(r)
		}
	}
	s = buf.String()

	// Keep letters and digits (including non-ASCII scripts), turn whitespace
	// and dashes into a single separator, drop everything else.
	buf.Reset()
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			buf.WriteRune(r)
		case unicode.IsSpace(r) || r == '-':
			buf.WriteByte('-')
		}
	}
	s = buf.String()

	// Collapse multiple dashes
	s = multiDash.ReplaceAllString(s, "-")

	// Truncate to 40 characters (runes, not bytes)
	if runes := []rune(s); len(runes) > 40 {
		s = string(runes[:40])
	}

	// Strip leading and trailing dashes
	s = strings.Trim(s, "-")

	return s
}

func Filename(id int, slug string) string {
	if slug == "" {
		slug = "issue"
	}
	return fmt.Sprintf("%04d-%s.md", id, slug)
}
