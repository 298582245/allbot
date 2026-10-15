package router

import (
	"testing"

	"github.com/allbot/allbot/core/config"
)

func TestNormalizeFakeMessageIdentitySupportsUnionIDAndQQGroupTarget(t *testing.T) {
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	account, err := database.EnsureUserAccount("qq_office", "member-openid")
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(nil)
	router.SetDatabase(database)

	userID, groupID := router.normalizeFakeMessageIdentity("qq_office", account.UnionID, "group-openid")
	if userID != account.UserID || groupID != "group-openid" {
		t.Fatalf("normalized identity = user=%q group=%q", userID, groupID)
	}

	userID, groupID = router.normalizeFakeMessageIdentity("qq_office", account.UnionID, "group_group-openid")
	if userID != account.UserID || groupID != "group-openid" {
		t.Fatalf("prefixed normalized identity = user=%q group=%q", userID, groupID)
	}
}

func TestNormalizeFakeMessageIdentityKeepsUnknownNativeIDs(t *testing.T) {
	router := NewRouter(nil)
	userID, groupID := router.normalizeFakeMessageIdentity("qq_office", "unknown-user", "group_unknown")
	if userID != "unknown-user" || groupID != "unknown" {
		t.Fatalf("unknown identity = user=%q group=%q", userID, groupID)
	}
}
