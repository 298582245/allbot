package router

import (
	"testing"

	"github.com/allbot/allbot/core/types"
)

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

func TestAllowSystemHardBlockUnionID(t *testing.T) {
	message := &types.Message{Platform: "qq", UserID: "user-1"}
	if allowSystemHardBlock(types.AccessControlConfig{BlockedUnionIDs: []string{"union-1"}}, message, "union-1") {
		t.Fatal("expected matched union_id blacklist to hard block system message")
	}
	if !allowSystemHardBlock(types.AccessControlConfig{BlockedUnionIDs: []string{"union-2"}}, message, "union-1") {
		t.Fatal("expected unmatched union_id blacklist to allow system message")
	}
}
