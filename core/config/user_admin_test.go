package config

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestUsersMigrationBackfillsAndPreservesLegacyDuplicates(t *testing.T) {
	db := newUserAdminTestDatabase(t)
	legacy, err := db.EnsureUserAccount("telegram", "legacy-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`DROP TRIGGER trg_user_accounts_prevent_union_platform_duplicate_insert`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO user_accounts (platform, user_id, union_id) VALUES ('telegram', 'legacy-2', ?)`, legacy.UnionID); err != nil {
		t.Fatal(err)
	}
	if err := migrateUsersTable(db.db); err != nil {
		t.Fatalf("migrateUsersTable returned error: %v", err)
	}
	accounts, _, err := db.ListUserAccounts(UserQuery{}, legacy.UnionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || !accounts[0].DuplicatePlatform || !accounts[1].DuplicatePlatform {
		t.Fatalf("历史重复账号应保留并标记: %#v", accounts)
	}
	if _, err := db.db.Exec(`INSERT INTO user_accounts (platform, user_id, union_id) VALUES ('telegram', 'legacy-3', ?)`, legacy.UnionID); err == nil {
		t.Fatal("新同平台账号应被触发器拒绝")
	}
}

func TestUserAdminListDetailAndAtomicPointAdjustment(t *testing.T) {
	db := newUserAdminTestDatabase(t)
	account, err := db.EnsureUserAccount("qq", "user_100%")
	if err != nil {
		t.Fatal(err)
	}
	users, total, err := db.ListUsers(UserQuery{Keyword: "user_100%", Platform: "qq"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(users) != 1 || users[0].UnionID != account.UnionID || users[0].AccountCount != 1 || users[0].PlatformCount != 1 {
		t.Fatalf("unexpected users total=%d items=%#v", total, users)
	}
	if unmatched, unmatchedTotal, err := db.ListUsers(UserQuery{Keyword: "userX100%"}); err != nil || unmatchedTotal != 0 || len(unmatched) != 0 {
		t.Fatalf("LIKE 通配符应按字面值搜索: total=%d items=%#v err=%v", unmatchedTotal, unmatched, err)
	}
	detail, err := db.GetUserDetail(account.UnionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Accounts) != 1 || detail.Accounts[0].UserID != "user_100%" {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	adjustment, err := db.AdjustUserPoints(account.UnionID, 25, "后台调整")
	if err != nil {
		t.Fatal(err)
	}
	if adjustment.Delta != 25 || adjustment.BalanceAfter != 25 || adjustment.Transaction.Source != "admin_adjustment" || adjustment.Transaction.SourceID == "" {
		t.Fatalf("unexpected adjustment: %#v", adjustment)
	}
	if _, err := db.AdjustUserPoints(account.UnionID, -30, "超额扣减"); err == nil {
		t.Fatal("超额扣减应失败")
	}
	points, err := db.GetUserPoints(account.UnionID)
	if err != nil || points != 25 {
		t.Fatalf("points=%d err=%v", points, err)
	}
	transactions, transactionTotal, err := db.ListPointTransactions(PointTransactionQuery{UnionID: account.UnionID, Source: "admin_adjustment"})
	if err != nil || transactionTotal != 1 || len(transactions) != 1 {
		t.Fatalf("unexpected transactions total=%d items=%#v err=%v", transactionTotal, transactions, err)
	}
	if _, err := db.AdjustUserPoints("missing", 1, "未知用户"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("未知用户不应被隐式创建: %v", err)
	}
}

func TestSetUserDisabledKeepsWebLocalDisabledIndependent(t *testing.T) {
	db, webUser := newWebChatPasswordResetTestUser(t)
	session, err := db.CreateWebChatSession(webUser.UserID, "agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetUserDisabled(webUser.UnionID, true); err != nil {
		t.Fatal(err)
	}
	disabled, err := db.IsUserDisabled(WebChatPlatform, webUser.UserID)
	if err != nil || !disabled {
		t.Fatalf("disabled=%v err=%v", disabled, err)
	}
	if _, err := db.GetWebChatSession(session.Token); err != sql.ErrNoRows {
		t.Fatalf("封禁后 session 应失效，实际 %v", err)
	}
	if _, err := db.VerifyWebChatLogin(webUser.Email, "oldpassword123"); err == nil {
		t.Fatal("统一封禁后密码登录应失败")
	}
	if _, err := db.db.Exec(`UPDATE web_chat_users SET disabled = 1 WHERE user_id = ?`, webUser.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetUserDisabled(webUser.UnionID, false); err != nil {
		t.Fatal(err)
	}
	var localDisabled int
	if err := db.db.QueryRow(`SELECT disabled FROM web_chat_users WHERE user_id = ?`, webUser.UserID).Scan(&localDisabled); err != nil {
		t.Fatal(err)
	}
	if localDisabled != 1 {
		t.Fatal("统一解禁不应覆盖 Web Chat 独立禁用")
	}
	if _, err := db.VerifyWebChatLogin(webUser.Email, "oldpassword123"); err == nil {
		t.Fatal("Web Chat 独立禁用仍应阻止登录")
	}
}

func TestCreateUserBindCodeReusesUnexpiredCode(t *testing.T) {
	db := newUserAdminTestDatabase(t)

	first, err := db.CreateUserBindCode("telegram", "source-user")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreateUserBindCode("telegram", "source-user")
	if err != nil {
		t.Fatal(err)
	}
	if second.Code != first.Code {
		t.Fatalf("有效期内应复用绑定码: first=%s second=%s", first.Code, second.Code)
	}
	if !second.ExpiresAt.Equal(first.ExpiresAt) || !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("复用绑定码不应刷新有效期: first=%#v second=%#v", first, second)
	}

	if _, err := db.db.Exec(`UPDATE user_bind_codes SET expires_at = datetime('now', '-1 minute') WHERE code = ?`, first.Code); err != nil {
		t.Fatal(err)
	}
	third, err := db.CreateUserBindCode("telegram", "source-user")
	if err != nil {
		t.Fatal(err)
	}
	if !third.ExpiresAt.After(time.Now()) || third.CreatedAt.Before(first.CreatedAt) {
		t.Fatalf("过期后应生成新的有效绑定码: %#v", third)
	}
	var activeCount int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM user_bind_codes WHERE platform = ? AND user_id = ? AND expires_at > CURRENT_TIMESTAMP`, "telegram", "source-user").Scan(&activeCount); err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 {
		t.Fatalf("同一用户应仅保留一个有效绑定码，实际 %d 个", activeCount)
	}
}

func TestCreateUserBindCodeConcurrentCallsReuseCode(t *testing.T) {
	db := newUserAdminTestDatabase(t)
	const calls = 8
	start := make(chan struct{})
	results := make(chan struct {
		code string
		err  error
	}, calls)
	for range calls {
		go func() {
			<-start
			code, err := db.CreateUserBindCode("telegram", "concurrent-user")
			result := struct {
				code string
				err  error
			}{err: err}
			if code != nil {
				result.code = code.Code
			}
			results <- result
		}()
	}
	close(start)

	codes := make(map[string]struct{})
	for range calls {
		result := <-results
		if result.err != nil {
			t.Fatalf("CreateUserBindCode returned error: %v", result.err)
		}
		codes[result.code] = struct{}{}
	}
	if len(codes) != 1 {
		t.Fatalf("并发获取应返回同一个绑定码: %#v", codes)
	}
	var activeCount int
	if err := db.db.QueryRow(`SELECT COUNT(*) FROM user_bind_codes WHERE platform = ? AND user_id = ? AND expires_at > CURRENT_TIMESTAMP`, "telegram", "concurrent-user").Scan(&activeCount); err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 {
		t.Fatalf("并发获取后应仅有一个有效绑定码，实际 %d 个", activeCount)
	}
}

func TestBindUserByCodeRejectsOverlappingPlatformSets(t *testing.T) {
	db := newUserAdminTestDatabase(t)
	source, err := db.EnsureUserAccount("telegram", "source-tg")
	if err != nil {
		t.Fatal(err)
	}
	target, err := db.EnsureUserAccount("qq", "target-qq")
	if err != nil {
		t.Fatal(err)
	}
	secondTarget, err := db.EnsureUserAccount("telegram", "target-tg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`DROP TRIGGER trg_user_accounts_prevent_union_platform_duplicate_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`UPDATE user_accounts SET union_id = ? WHERE id = ?`, target.UnionID, secondTarget.ID); err != nil {
		t.Fatal(err)
	}
	if err := migrateUsersTable(db.db); err != nil {
		t.Fatal(err)
	}
	code, err := db.CreateUserBindCode(source.Platform, source.UserID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.BindUserByCode("qq", "target-qq", code.Code)
	if !errors.Is(err, ErrUserBindingConflict) {
		t.Fatalf("expected binding conflict, got %v", err)
	}
}

func TestBindUserByCodeMigratesUnionOwnedData(t *testing.T) {
	db := newUserAdminTestDatabase(t)
	source, err := db.EnsureUserAccount("telegram", "source-data")
	if err != nil {
		t.Fatal(err)
	}
	target, err := db.EnsureUserAccount("qq", "target-data")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO point_transactions (union_id, delta, balance_after, source, source_id) VALUES (?, 1, 1, 'test', 'legacy')`, target.UnionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO payment_orders (order_no, union_id, subject, amount_cents, points_amount, provider, method, status, expired_at) VALUES ('legacy-order', ?, 'test', 100, 1, 'test', 'test', 'pending', datetime('now', '+1 day'))`, target.UnionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO plugin_accounts (plugin_id, union_id) VALUES ('plugin', ?)`, target.UnionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO plugin_authorizations (plugin_id, union_id) VALUES ('plugin', ?)`, target.UnionID); err != nil {
		t.Fatal(err)
	}
	code, err := db.CreateUserBindCode(source.Platform, source.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.BindUserByCode(target.Platform, target.UserID, code.Code); err != nil {
		t.Fatal(err)
	}
	for table, key := range map[string]string{
		"point_transactions":    "source_id = 'legacy'",
		"payment_orders":        "order_no = 'legacy-order'",
		"plugin_accounts":       "plugin_id = 'plugin'",
		"plugin_authorizations": "plugin_id = 'plugin'",
	} {
		var unionID string
		if err := db.db.QueryRow(`SELECT union_id FROM ` + table + ` WHERE ` + key).Scan(&unionID); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if unionID != source.UnionID {
			t.Fatalf("%s union_id=%s, want %s", table, unionID, source.UnionID)
		}
	}
}

func TestBindUserByCodeMergesDisabledWithOR(t *testing.T) {
	db := newUserAdminTestDatabase(t)
	source, err := db.EnsureUserAccount("telegram", "source")
	if err != nil {
		t.Fatal(err)
	}
	target, err := db.EnsureUserAccount("qq", "target")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetUserDisabled(target.UnionID, true); err != nil {
		t.Fatal(err)
	}
	code, err := db.CreateUserBindCode(source.Platform, source.UserID)
	if err != nil {
		t.Fatal(err)
	}
	bound, _, err := db.BindUserByCode(target.Platform, target.UserID, code.Code)
	if err != nil {
		t.Fatal(err)
	}
	if bound.UnionID != source.UnionID {
		t.Fatalf("expected source union %s, got %s", source.UnionID, bound.UnionID)
	}
	disabled, err := db.IsUnionDisabled(source.UnionID)
	if err != nil || !disabled {
		t.Fatalf("merged union should remain disabled: disabled=%v err=%v", disabled, err)
	}
	if _, err := db.GetUserDetail(target.UnionID); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("merged user should be removed, got %v", err)
	}
}

func TestReplaceWithMigratesLegacyDatabase(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.db")
	legacyPath := filepath.Join(dir, "legacy.db")
	db, err := NewDatabase(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	legacy, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`
		CREATE TABLE user_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT NOT NULL,
			user_id TEXT NOT NULL,
			union_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(platform, user_id)
		);
		CREATE TABLE user_points (
			union_id TEXT PRIMARY KEY,
			points INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO user_accounts (platform, user_id, union_id) VALUES ('qq', 'legacy', 'legacy-union');
	`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceWith(legacyPath); err != nil {
		t.Fatal(err)
	}
	detail, err := db.GetUserDetail("legacy-union")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Accounts) != 1 || detail.Accounts[0].UserID != "legacy" {
		t.Fatalf("unexpected restored detail: %#v", detail)
	}
}

func newUserAdminTestDatabase(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
