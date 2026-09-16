package contract

import (
	"strings"
	"testing"
)

func TestSplitTextMessageLoopsUntilAllContentSent(t *testing.T) {
	text := strings.Repeat("一", DefaultTextMessageLimit*2+17)
	parts := SplitTextMessage(text, DefaultTextMessageLimit)
	if len(parts) != 3 {
		t.Fatalf("parts length = %d", len(parts))
	}
	if got := strings.Join(parts, ""); got != text {
		t.Fatal("joined parts do not match original text")
	}
	for index, part := range parts {
		if count := TextRuneCount(part); count > DefaultTextMessageLimit {
			t.Fatalf("part %d length = %d", index, count)
		}
	}
}

func TestSplitTextMessagePrefersNewline(t *testing.T) {
	text := strings.Repeat("a", 3900) + "\n" + strings.Repeat("b", 200)
	parts := SplitTextMessage(text, 4000)
	if len(parts) != 2 {
		t.Fatalf("parts = %#v", parts)
	}
	if strings.Contains(parts[0], "b") || !strings.HasPrefix(parts[1], "b") {
		t.Fatalf("unexpected newline split: %#v", parts)
	}
}

func TestSplitTextMessagePreserveCQKeepsCodeIntact(t *testing.T) {
	cq := "[CQ:at,qq=123456]"
	text := strings.Repeat("a", 3990) + cq + strings.Repeat("b", 50)
	parts := SplitTextMessagePreserveCQ(text, DefaultTextMessageLimit)
	if len(parts) != 2 {
		t.Fatalf("parts length = %d", len(parts))
	}
	if strings.Contains(parts[0], "[CQ:") || !strings.HasPrefix(parts[1], cq) {
		t.Fatalf("CQ code was split: %#v", parts)
	}
	if got := strings.Join(parts, ""); got != text {
		t.Fatal("joined parts do not match original text")
	}
}
