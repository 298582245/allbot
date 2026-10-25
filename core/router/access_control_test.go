package router

import (
	"regexp"
	"testing"

	"github.com/allbot/allbot/core/types"
)

func TestRouterHasWhitelistBlockedPlugin(t *testing.T) {
	router := NewRouter(nil)
	router.plugins["demo"] = &types.Plugin{
		Enabled:       true,
		Platforms:     []string{"qq_office"},
		TriggerRegex:  regexp.MustCompile(`^朴朴登录$`),
		AccessControl: types.AccessControlConfig{WhitelistGroups: []string{"allowed-group"}},
	}

	blocked := &types.Message{Platform: "qq_office", GroupID: "blocked-group", UserID: "user-1", Content: "朴朴登录"}
	if !router.hasWhitelistBlockedPlugin(blocked) {
		t.Fatal("expected matching plugin blocked by group whitelist")
	}

	allowed := &types.Message{Platform: "qq_office", GroupID: "allowed-group", UserID: "user-1", Content: "朴朴登录"}
	if router.hasWhitelistBlockedPlugin(allowed) {
		t.Fatal("expected allowed group not to be reported as whitelist-blocked")
	}

	nonMatching := &types.Message{Platform: "qq_office", GroupID: "blocked-group", UserID: "user-1", Content: "其他指令"}
	if router.hasWhitelistBlockedPlugin(nonMatching) {
		t.Fatal("expected non-matching trigger not to be reported as whitelist-blocked")
	}
}

func TestAllowMessageByAccessControlUnionID(t *testing.T) {
	message := &types.Message{Platform: "qq", UserID: "user-1", GroupID: "group-1"}

	if allowMessageByAccessControl(types.AccessControlConfig{WhitelistUnionIDs: []string{"union-2"}}, message, "union-1") {
		t.Fatal("expected unmatched union_id whitelist to block message")
	}
	if !allowMessageByAccessControl(types.AccessControlConfig{WhitelistUnionIDs: []string{"union-1"}}, message, "union-1") {
		t.Fatal("expected matched union_id whitelist to allow message")
	}
	if allowMessageByAccessControl(types.AccessControlConfig{BlockedUnionIDs: []string{"union-1"}}, message, "union-1") {
		t.Fatal("expected matched union_id blacklist to block message")
	}
	if allowMessageByAccessControl(types.AccessControlConfig{WhitelistUnionIDs: []string{"union-1"}}, message, "") {
		t.Fatal("expected union_id whitelist to block unregistered user without union_id")
	}
}

func TestAllowMessageByAccessControlAdminOnly(t *testing.T) {
	message := &types.Message{Platform: "qq", UserID: "user-1", GroupID: "group-1"}
	config := types.AccessControlConfig{AdminOnly: true}

	if !allowMessageByAccessControlWithAdmin(config, message, "", true) {
		t.Fatal("expected administrator to bypass admin-only access control")
	}
	if allowMessageByAccessControlWithAdmin(config, message, "", false) {
		t.Fatal("expected non-administrator without personal whitelist to be blocked")
	}
	config.WhitelistUserIDs = []string{"user-1"}
	if !allowMessageByAccessControlWithAdmin(config, message, "", false) {
		t.Fatal("expected personal whitelist user to bypass admin-only access control")
	}
	config.BlockedUserIDs = []string{"user-1"}
	if allowMessageByAccessControlWithAdmin(config, message, "", true) {
		t.Fatal("expected blocked user to remain blocked even when administrator")
	}
	config.BlockedUserIDs = nil
	config.WhitelistUserIDs = nil
	config.WhitelistGroups = []string{"other-group"}
	if allowMessageByAccessControlWithAdmin(config, message, "", false) {
		t.Fatal("expected group whitelist not to act as admin-only exception")
	}
}

func TestAllowSystemHardBlockUnionID(t *testing.T) {
	message := &types.Message{Platform: "qq", UserID: "user-1"}
	if allowSystemHardBlock(types.AccessControlConfig{BlockedUnionIDs: []string{"union-1"}}, message, "union-1") {
		t.Fatal("expected matched union_id blacklist to hard block system message")
	}
	if !allowSystemHardBlock(types.AccessControlConfig{BlockedUnionIDs: []string{"union-2"}}, message, "union-1") {
		t.Fatal("expected unmatched union_id blacklist to allow system message")
	}
}
