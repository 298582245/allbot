package config

import "testing"

func TestBuiltinRestartKeywordReplySeeded(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	items, err := db.ListKeywordReplies()
	if err != nil {
		t.Fatalf("ListKeywordReplies returned error: %v", err)
	}

	var restart *KeywordReply
	for _, item := range items {
		if item.Keyword == "重启" {
			restart = item
			break
		}
	}
	if restart == nil {
		t.Fatal("builtin keyword 重启 not found")
	}
	if !restart.Builtin {
		t.Fatal("重启 should be builtin")
	}
	if !restart.AdminOnly {
		t.Fatal("重启 should be admin only")
	}
	if !restart.Pinned {
		t.Fatal("重启 should be pinned")
	}
	if restart.MatchType != "exact" {
		t.Fatalf("MatchType = %q, expected exact", restart.MatchType)
	}
	if restart.ReplyType != "builtin" {
		t.Fatalf("ReplyType = %q, expected builtin", restart.ReplyType)
	}
	if !restart.Enabled {
		t.Fatal("重启 should be enabled")
	}
}
