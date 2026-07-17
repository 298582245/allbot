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

func TestBuiltinKeywordRepliesUseCurrentContentAsTrigger(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	items, err := db.ListKeywordReplies()
	if err != nil {
		t.Fatalf("ListKeywordReplies returned error: %v", err)
	}

	cases := []struct {
		keyword   string
		content   string
		matchType string
	}{
		{keyword: "myid", content: "myid", matchType: "exact"},
		{keyword: "我的平台", content: "我的平台", matchType: "exact"},
		{keyword: "groupId", content: "groupId", matchType: "exact"},
		{keyword: `(?i)^v(ersion)?$`, content: "version", matchType: "regex"},
	}
	for _, tc := range cases {
		var item *KeywordReply
		for _, candidate := range items {
			if candidate.Content == tc.content {
				item = candidate
				break
			}
		}
		if item == nil {
			t.Fatalf("builtin keyword with content %s not found", tc.content)
		}
		if item.Content != tc.content {
			t.Fatalf("keyword %s content = %q, expected %q", tc.keyword, item.Content, tc.content)
		}
		if item.MatchType != tc.matchType {
			t.Fatalf("keyword %s match_type = %q, expected %q", tc.keyword, item.MatchType, tc.matchType)
		}
		if tc.keyword == `^[Vv]ersion?$` && item.Keyword != tc.keyword {
			t.Fatalf("keyword version regex = %q, expected %q", item.Keyword, tc.keyword)
		}
	}
}

func TestBuiltinBotIDKeywordReplySeededIdempotently(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := ensureBuiltinKeywordReplies(db.db); err != nil {
		t.Fatalf("second ensureBuiltinKeywordReplies returned error: %v", err)
	}
	items, err := db.ListKeywordReplies()
	if err != nil {
		t.Fatalf("ListKeywordReplies returned error: %v", err)
	}
	var botID *KeywordReply
	count := 0
	for _, item := range items {
		if item.Keyword == "botid" || item.Content == "botid" {
			botID = item
			count++
		}
	}
	if count != 1 || botID == nil {
		t.Fatalf("botid seed count = %d", count)
	}
	if !botID.AdminOnly || !botID.Builtin || !botID.Pinned || !botID.Enabled {
		t.Fatalf("botid flags unexpected: admin=%v builtin=%v pinned=%v enabled=%v", botID.AdminOnly, botID.Builtin, botID.Pinned, botID.Enabled)
	}
	if botID.MatchType != "exact" || botID.ReplyType != "builtin" || botID.Content != "botid" {
		t.Fatalf("botid seed unexpected: match=%q reply=%q content=%q", botID.MatchType, botID.ReplyType, botID.Content)
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

func TestBuiltinUserSearchKeywordReplySeeded(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	items, err := db.ListKeywordReplies()
	if err != nil {
		t.Fatalf("ListKeywordReplies returned error: %v", err)
	}

	var userSearch *KeywordReply
	for _, item := range items {
		if item.Keyword == "用户搜索" {
			userSearch = item
			break
		}
	}
	if userSearch == nil {
		t.Fatal("builtin keyword 用户搜索 not found")
	}
	if !userSearch.Builtin || !userSearch.AdminOnly || !userSearch.Pinned || !userSearch.Enabled {
		t.Fatalf("用户搜索 flags unexpected: builtin=%v admin=%v pinned=%v enabled=%v", userSearch.Builtin, userSearch.AdminOnly, userSearch.Pinned, userSearch.Enabled)
	}
	if userSearch.MatchType != "exact" || userSearch.ReplyType != "builtin" {
		t.Fatalf("用户搜索 type unexpected: match=%q reply=%q", userSearch.MatchType, userSearch.ReplyType)
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
