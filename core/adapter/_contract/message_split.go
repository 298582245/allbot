package contract

import (
	"strings"
	"unicode/utf8"
)

const (
	DefaultTextMessageLimit = 4000
	DefaultRichMessageLimit = 10000
)

func TextRuneCount(text string) int {
	return len([]rune(text))
}

func SplitTextMessage(text string, limit int) []string {
	return splitTextMessage(text, limit, false)
}

func SplitTextMessagePreserveCQ(text string, limit int) []string {
	return splitTextMessage(text, limit, true)
}

func SplitMarkdownMessage(markdown string, limit int) []string {
	if limit <= 0 || TextRuneCount(markdown) <= limit {
		if markdown == "" {
			return nil
		}
		return []string{markdown}
	}
	units := splitMarkdownUnits(markdown)
	parts := make([]string, 0, TextRuneCount(markdown)/limit+1)
	var builder strings.Builder
	count := 0
	lastBreakByte := -1
	lastBreakCount := 0
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		parts = append(parts, builder.String())
		builder.Reset()
		count = 0
		lastBreakByte = -1
		lastBreakCount = 0
	}
	for _, unit := range units {
		unitCount := TextRuneCount(unit.text)
		if unitCount > limit {
			flush()
			parts = append(parts, splitOversizeMarkdownUnit(unit, limit)...)
			continue
		}
		if count+unitCount > limit {
			if lastBreakByte > 0 {
				current := builder.String()
				part := strings.TrimRight(current[:lastBreakByte], "\r\n")
				if part != "" {
					parts = append(parts, part)
				}
				remaining := strings.TrimLeft(current[lastBreakByte:], "\r\n")
				builder.Reset()
				builder.WriteString(remaining)
				count = count - lastBreakCount
				lastBreakByte = -1
				lastBreakCount = 0
				if count+unitCount > limit {
					flush()
				}
			} else {
				flush()
			}
		}
		builder.WriteString(unit.text)
		count += unitCount
		if unit.breakAfter {
			lastBreakByte = builder.Len()
			lastBreakCount = count
		}
	}
	flush()
	return compactTextParts(parts)
}

func splitTextMessage(text string, limit int, preserveCQ bool) []string {
	if limit <= 0 || TextRuneCount(text) <= limit {
		if text == "" {
			return nil
		}
		return []string{text}
	}
	units := splitMessageUnits(text, preserveCQ)
	parts := make([]string, 0, TextRuneCount(text)/limit+1)
	var builder strings.Builder
	count := 0
	lastBreakByte := -1
	lastBreakCount := 0
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		parts = append(parts, builder.String())
		builder.Reset()
		count = 0
		lastBreakByte = -1
		lastBreakCount = 0
	}
	for _, unit := range units {
		unitCount := TextRuneCount(unit)
		if unitCount > limit {
			flush()
			parts = append(parts, splitOversizeUnit(unit, limit)...)
			continue
		}
		if count+unitCount > limit {
			if lastBreakByte > 0 {
				current := builder.String()
				parts = append(parts, strings.TrimRight(current[:lastBreakByte], "\r\n"))
				remaining := strings.TrimLeft(current[lastBreakByte:], "\r\n")
				builder.Reset()
				builder.WriteString(remaining)
				count = count - lastBreakCount
				lastBreakByte = -1
				lastBreakCount = 0
				if count+unitCount > limit {
					flush()
				}
			} else {
				flush()
			}
		}
		builder.WriteString(unit)
		count += unitCount
		if unit == "\n" || unit == "\r\n" {
			lastBreakByte = builder.Len()
			lastBreakCount = count
		}
	}
	flush()
	return compactTextParts(parts)
}

type markdownUnit struct {
	text       string
	breakAfter bool
	kind       string
	delimiter  string
}

func splitMarkdownUnits(text string) []markdownUnit {
	units := make([]markdownUnit, 0, len(text))
	for i := 0; i < len(text); {
		if text[i] == '\r' || text[i] == '\n' {
			size := 1
			if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
				size = 2
			}
			units = append(units, markdownUnit{text: text[i : i+size], breakAfter: true})
			i += size
			continue
		}
		if text[i] == '\\' && i+1 < len(text) {
			_, size := utf8.DecodeRuneInString(text[i+1:])
			units = append(units, markdownUnit{text: text[i : i+1+size]})
			i += 1 + size
			continue
		}
		if isMarkdownLineStart(text, i) {
			if end, marker := markdownFenceEnd(text, i); end > i {
				units = append(units, markdownUnit{text: text[i:end], kind: "fence", delimiter: marker})
				i = end
				continue
			}
		}
		if end := markdownLinkEnd(text, i); end > i {
			units = append(units, markdownUnit{text: text[i:end], kind: "link"})
			i = end
			continue
		}
		if end, delimiter := markdownDelimitedEnd(text, i); end > i {
			units = append(units, markdownUnit{text: text[i:end], kind: "delimited", delimiter: delimiter})
			i = end
			continue
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		units = append(units, markdownUnit{text: text[i : i+size]})
		i += size
	}
	return units
}

func isMarkdownLineStart(text string, index int) bool {
	return index == 0 || text[index-1] == '\n'
}

func markdownFenceEnd(text string, start int) (int, string) {
	lineEnd := strings.IndexByte(text[start:], '\n')
	if lineEnd < 0 {
		return 0, ""
	}
	lineEnd += start
	line := strings.TrimLeft(text[start:lineEnd], " ")
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return 0, ""
	}
	markerChar := line[0]
	markerLength := 0
	for markerLength < len(line) && line[markerLength] == markerChar {
		markerLength++
	}
	if markerLength < 3 {
		return 0, ""
	}
	marker := strings.Repeat(string(markerChar), markerLength)
	search := lineEnd + 1
	for search < len(text) {
		next := strings.IndexByte(text[search:], '\n')
		end := len(text)
		if next >= 0 {
			end = search + next
		}
		candidate := strings.TrimSpace(text[search:end])
		if strings.HasPrefix(candidate, marker) {
			rest := strings.TrimSpace(strings.TrimPrefix(candidate, marker))
			if rest == "" {
				if end < len(text) {
					end++
				}
				return end, marker
			}
		}
		if next < 0 {
			break
		}
		search = end + 1
	}
	return 0, ""
}

func markdownLinkEnd(text string, start int) int {
	if start < len(text) && text[start] == '<' {
		end := strings.IndexByte(text[start+1:], '>')
		if end >= 0 {
			end += start + 2
			url := strings.ToLower(text[start+1 : end-1])
			if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
				return end
			}
		}
		return 0
	}
	labelStart := start
	if start < len(text) && text[start] == '!' {
		labelStart++
	}
	if labelStart >= len(text) || text[labelStart] != '[' || (start > 0 && text[start-1] == '\\') {
		return 0
	}
	labelEnd := markdownMatchingDelimiter(text, labelStart+1, '[', ']')
	if labelEnd < 0 || labelEnd+1 >= len(text) || text[labelEnd+1] != '(' {
		return 0
	}
	destinationEnd := markdownMatchingDelimiter(text, labelEnd+2, '(', ')')
	if destinationEnd < 0 {
		return 0
	}
	return destinationEnd + 1
}

func markdownDelimitedEnd(text string, start int) (int, string) {
	if start > 0 && text[start-1] == '\\' {
		return 0, ""
	}
	delimiters := []string{"```", "~~~", "**", "__", "~~", "`", "*", "_"}
	for _, delimiter := range delimiters {
		if !strings.HasPrefix(text[start:], delimiter) {
			continue
		}
		end := strings.Index(text[start+len(delimiter):], delimiter)
		if end < 0 {
			continue
		}
		end += start + len(delimiter)*2
		return end, delimiter
	}
	return 0, ""
}

func markdownMatchingDelimiter(text string, start int, opening, closing byte) int {
	depth := 1
	for i := start; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		switch text[i] {
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitOversizeMarkdownUnit(unit markdownUnit, limit int) []string {
	if unit.kind == "link" {
		plain := unit.text
		if strings.HasPrefix(plain, "<") && strings.HasSuffix(plain, ">") {
			plain = plain[1 : len(plain)-1]
		} else {
			labelStart := 0
			if strings.HasPrefix(plain, "!") {
				labelStart = 1
			}
			labelEnd := markdownMatchingDelimiter(plain, labelStart+1, '[', ']')
			if labelEnd >= 0 && labelEnd+1 < len(plain) && plain[labelEnd+1] == '(' && strings.HasSuffix(plain, ")") {
				plain = plain[labelStart+1:labelEnd] + " (" + plain[labelEnd+2:len(plain)-1] + ")"
			}
		}
		return splitOversizeUnit(plain, limit)
	}
	if unit.kind == "delimited" && unit.delimiter != "" {
		overhead := TextRuneCount(unit.delimiter) * 2
		if overhead < limit {
			inner := unit.text[len(unit.delimiter):]
			inner = inner[:len(inner)-len(unit.delimiter)]
			chunks := splitOversizeUnit(inner, limit-overhead)
			parts := make([]string, 0, len(chunks))
			for _, chunk := range chunks {
				parts = append(parts, unit.delimiter+chunk+unit.delimiter)
			}
			return parts
		}
	}
	if unit.kind == "fence" && unit.delimiter != "" {
		if openingEnd := strings.IndexByte(unit.text, '\n'); openingEnd >= 0 {
			opening := unit.text[:openingEnd+1]
			closingStart := -1
			for lineStart := openingEnd + 1; lineStart < len(unit.text); {
				lineEnd := strings.IndexByte(unit.text[lineStart:], '\n')
				if lineEnd < 0 {
					lineEnd = len(unit.text)
				} else {
					lineEnd += lineStart
				}
				candidate := strings.TrimSpace(unit.text[lineStart:lineEnd])
				if candidate == unit.delimiter {
					closingStart = lineStart
					break
				}
				if lineEnd >= len(unit.text) {
					break
				}
				lineStart = lineEnd + 1
			}
			if closingStart > openingEnd && TextRuneCount(opening)+TextRuneCount(unit.delimiter) < limit {
				body := unit.text[openingEnd+1 : closingStart]
				chunks := splitOversizeUnit(body, limit-TextRuneCount(opening)-TextRuneCount(unit.delimiter))
				parts := make([]string, 0, len(chunks))
				for _, chunk := range chunks {
					parts = append(parts, opening+chunk+unit.delimiter)
				}
				return parts
			}
		}
	}
	return splitOversizeUnit(unit.text, limit)
}

func splitMessageUnits(text string, preserveCQ bool) []string {
	units := make([]string, 0, len(text))
	for i := 0; i < len(text); {
		if preserveCQ && strings.HasPrefix(text[i:], "[CQ:") {
			if end := strings.IndexByte(text[i:], ']'); end >= 0 {
				units = append(units, text[i:i+end+1])
				i += end + 1
				continue
			}
		}
		if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
			units = append(units, "\r\n")
			i += 2
			continue
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		units = append(units, text[i:i+size])
		i += size
	}
	return units
}

func splitOversizeUnit(text string, limit int) []string {
	runes := []rune(text)
	parts := make([]string, 0, len(runes)/limit+1)
	for len(runes) > 0 {
		end := limit
		if len(runes) < end {
			end = len(runes)
		}
		parts = append(parts, string(runes[:end]))
		runes = runes[end:]
	}
	return parts
}

func compactTextParts(parts []string) []string {
	compacted := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			compacted = append(compacted, part)
		}
	}
	return compacted
}
