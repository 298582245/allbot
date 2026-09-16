package contract

import (
	"strings"
	"unicode/utf8"
)

const DefaultTextMessageLimit = 4000

func TextRuneCount(text string) int {
	return len([]rune(text))
}

func SplitTextMessage(text string, limit int) []string {
	return splitTextMessage(text, limit, false)
}

func SplitTextMessagePreserveCQ(text string, limit int) []string {
	return splitTextMessage(text, limit, true)
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
