package config

import "testing"

func TestBuiltinPluginListKeywordReplySeeded(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	items, err := db.ListKeywordReplies()
	if err != nil {
		t.Fatalf("ListKeywordReplies returned error: %v", err)
	}

	var pluginList *KeywordReply
	for _, item := range items {
		if item.Keyword == "插件列表" {
			pluginList = item
			break
		}
	}
	if pluginList == nil {
		t.Fatal("builtin keyword 插件列表 not found")
	}
	if !pluginList.Builtin || !pluginList.AdminOnly || !pluginList.Pinned || !pluginList.Enabled {
		t.Fatalf("插件列表 flags unexpected: builtin=%v admin=%v pinned=%v enabled=%v", pluginList.Builtin, pluginList.AdminOnly, pluginList.Pinned, pluginList.Enabled)
	}
	if pluginList.MatchType != "exact" || pluginList.ReplyType != "builtin" {
		t.Fatalf("插件列表 type unexpected: match=%q reply=%q", pluginList.MatchType, pluginList.ReplyType)
	}
}

func TestBuiltinRechargeKeywordReplyIsUserAvailable(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	items, err := db.ListKeywordReplies()
	if err != nil {
		t.Fatalf("ListKeywordReplies returned error: %v", err)
	}
	var recharge *KeywordReply
	for _, item := range items {
		if item.Keyword == "积分充值" {
			recharge = item
			break
		}
	}
	if recharge == nil {
		t.Fatal("builtin keyword 积分充值 not found")
	}
	if !recharge.Builtin || recharge.AdminOnly || !recharge.Pinned || !recharge.Enabled {
		t.Fatalf("积分充值 flags unexpected: builtin=%v admin=%v pinned=%v enabled=%v", recharge.Builtin, recharge.AdminOnly, recharge.Pinned, recharge.Enabled)
	}
}

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
