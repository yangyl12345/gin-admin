package retrieval

import (
	"strings"
	"unicode/utf8"
)

type TextChunk struct {
	Content   string
	LineStart int
	LineEnd   int
}

type lineUnit struct {
	text string
	line int
}

// ChunkText splits normalized UTF-8 text at headings, blank lines and line
// boundaries before falling back to rune boundaries for very long lines.
func ChunkText(raw string, maxChars, overlapChars int) []TextChunk {
	if maxChars <= 0 || overlapChars < 0 || overlapChars >= maxChars {
		return nil
	}
	raw = strings.TrimPrefix(strings.ReplaceAll(raw, "\r\n", "\n"), "\ufeff")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := strings.Split(raw, "\n")
	units := make([]lineUnit, 0, len(lines))
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) == 0 {
			units = append(units, lineUnit{line: i + 1})
			continue
		}
		for len(runes) > maxChars {
			units = append(units, lineUnit{text: string(runes[:maxChars]), line: i + 1})
			runes = runes[maxChars:]
		}
		units = append(units, lineUnit{text: string(runes), line: i + 1})
	}

	var result []TextChunk
	var current []lineUnit
	currentLen := 0
	flush := func() {
		for len(current) > 0 && strings.TrimSpace(current[0].text) == "" {
			current = current[1:]
		}
		for len(current) > 0 && strings.TrimSpace(current[len(current)-1].text) == "" {
			current = current[:len(current)-1]
		}
		if len(current) == 0 {
			currentLen = 0
			return
		}
		parts := make([]string, len(current))
		for i := range current {
			parts[i] = current[i].text
		}
		content := strings.TrimSpace(strings.Join(parts, "\n"))
		if content != "" {
			result = append(result, TextChunk{Content: content, LineStart: current[0].line, LineEnd: current[len(current)-1].line})
		}
		if overlapChars == 0 {
			current = nil
			currentLen = 0
			return
		}
		start, n := len(current), 0
		for start > 0 {
			unitLen := utf8.RuneCountInString(current[start-1].text)
			if n > 0 {
				unitLen++
			}
			if n+unitLen > overlapChars {
				break
			}
			start--
			n += unitLen
		}
		if start == len(current) && overlapChars > 0 {
			last := current[len(current)-1]
			runes := []rune(last.text)
			if len(runes) > overlapChars {
				runes = runes[len(runes)-overlapChars:]
			}
			current = []lineUnit{{text: string(runes), line: last.line}}
			currentLen = len(runes)
		} else {
			current = append([]lineUnit(nil), current[start:]...)
			currentLen = n
		}
	}

	for _, unit := range units {
		unitLen := utf8.RuneCountInString(unit.text)
		isHeading := strings.HasPrefix(strings.TrimSpace(unit.text), "#")
		if isHeading && len(current) > 0 && strings.TrimSpace(current[len(current)-1].text) != "" {
			flush()
		}
		additional := unitLen
		if len(current) > 0 {
			additional++
		}
		if len(current) > 0 && currentLen+additional > maxChars {
			flush()
			if len(current) > 0 && currentLen+additional > maxChars {
				current = nil
				currentLen = 0
				additional = unitLen
			}
		}
		current = append(current, unit)
		currentLen += additional
	}
	flush()
	return result
}
