package batch

import (
	"fmt"
	"strings"
)

// splitWords handles shell quoting without evaluating commands or expanding
// variables on the submission host.
func splitWords(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote byte
	started := false
	flush := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		if c == '\\' {
			if i+1 == len(line) {
				return nil, fmt.Errorf("trailing escape in command")
			}
			next := line[i+1]
			// In double quotes a backslash only escapes these shell characters.
			if quote == '"' && !strings.ContainsRune("$`\"\\", rune(next)) {
				word.WriteByte(c)
				continue
			}
			word.WriteByte(next)
			i++
			started = true
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			started = true
		case ' ', '\t':
			flush()
		default:
			word.WriteByte(c)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in command")
	}
	flush()
	return words, nil
}
