package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	qqofficeadapter "github.com/allbot/allbot/core/adapter/qq_office"
	"github.com/allbot/allbot/core/config"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/updater"
	"github.com/allbot/allbot/core/version"
)

type keywordReplyFakeAdapter struct {
	mu             sync.Mutex
	messages       []sentKeywordReplyMessage
	targetResolver adapter.ReplyTargetResolver
	textFormatter  adapter.ReplyTextFormatter
	sendResolver   adapter.SendTargetResolver
}

type sentKeywordReplyMessage struct {
	target string
	text   string
}

type fakeReleaseClient struct {
	release *updater.ReleaseInfo
	err     error
}

type fakeKeywordPluginAdminStore struct {
	plugins []*plugincore.PluginProcess
}

type fakeUpdateHandler struct {
	current updater.UpgradeState
	start   updater.UpgradeState
	err     error
	calls   int
}

func (h *fakeUpdateHandler) StartUpgrade(ctx context.Context) (updater.UpgradeState, error) {
	h.calls++
	return h.start, h.err
}

func (h *fakeUpdateHandler) CurrentState() updater.UpgradeState {
	return h.current
}

func (s *fakeKeywordPluginAdminStore) GetAllPlugins() []*plugincore.PluginProcess {
	return append([]*plugincore.PluginProcess(nil), s.plugins...)
}

func (s *fakeKeywordPluginAdminStore) TogglePlugin(pluginID string, enabled bool) error {
	for _, process := range s.plugins {
		if process != nil && process.Plugin != nil && process.Plugin.ID == pluginID {
			process.Plugin.Enabled = enabled
			return nil
		}
	}
	return errors.New("plugin not found")
}

func (s *fakeKeywordPluginAdminStore) SavePluginAccessControl(pluginID string, accessControl types.AccessControlConfig) error {
	for _, process := range s.plugins {
		if process != nil && process.Plugin != nil && process.Plugin.ID == pluginID {
			process.Plugin.AccessControl = accessControl
			return nil
		}
	}
	return errors.New("plugin not found")
}

func (c fakeReleaseClient) LatestRelease(ctx context.Context) (*updater.ReleaseInfo, error) {
	return c.release, c.err
}

func (a *keywordReplyFakeAdapter) GetPlatform() string { return "qq" }

func newReplyCapableKeywordReplyFakeAdapter(targetResolver adapter.ReplyTargetResolver, textFormatter adapter.ReplyTextFormatter) *keywordReplyFakeAdapter {
	fake := &keywordReplyFakeAdapter{targetResolver: targetResolver, textFormatter: textFormatter}
	if sendResolver, ok := targetResolver.(adapter.SendTargetResolver); ok {
		fake.sendResolver = sendResolver
	}
	return fake
}

func (a *keywordReplyFakeAdapter) ReplyTarget(msg *types.Message) string {
	if a.targetResolver == nil {
		return ""
	}
	return a.targetResolver.ReplyTarget(msg)
}

func (a *keywordReplyFakeAdapter) FormatReplyText(msg *types.Message, text string) string {
	if a.textFormatter == nil {
		return text
	}
	return a.textFormatter.FormatReplyText(msg, text)
}

func (a *keywordReplyFakeAdapter) SendTarget(userID string, groupID string) string {
	if a.sendResolver == nil {
		return ""
	}
	return a.sendResolver.SendTarget(userID, groupID)
}

func (a *keywordReplyFakeAdapter) SendMessage(target string, text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, sentKeywordReplyMessage{target: target, text: text})
	return nil
}

func (a *keywordReplyFakeAdapter) sentMessages() []sentKeywordReplyMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]sentKeywordReplyMessage(nil), a.messages...)
}

func (a *keywordReplyFakeAdapter) SendImage(target string, imageURL string) error { return nil }
func (a *keywordReplyFakeAdapter) SendFile(target string, filePath string) error  { return nil }
func (a *keywordReplyFakeAdapter) GetUserInfo(userID string) (*adapter.UserInfo, error) {
	return nil, nil
}
func (a *keywordReplyFakeAdapter) GetGroupInfo(groupID string) (*adapter.GroupInfo, error) {
	return nil, nil
}
func (a *keywordReplyFakeAdapter) AtUser(groupID string, userID string) error     { return nil }
func (a *keywordReplyFakeAdapter) Start() error                                   { return nil }
func (a *keywordReplyFakeAdapter) Stop() error                                    { return nil }
func (a *keywordReplyFakeAdapter) SetMessageHandler(handler func(*types.Message)) {}

func newKeywordReplyTestManager(t *testing.T, fake *keywordReplyFakeAdapter, admin bool) (*config.Database, *KeywordReplyManager) {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	manager := NewKeywordReplyManager(
		db,
		func(msg *types.Message) adapter.Adapter { return fake },
		func(platform, userID string) bool { return admin },
		time.Now(),
	)
	return db, manager
}

func TestKeywordReplyRegisterExistingUserRepliesAlreadyRegistered(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "telegram", UserID: "7089240306", Content: "注册"}) {
		t.Fatal("first register Handle returned false")
	}
	if !manager.Handle(&types.Message{Platform: "telegram", UserID: "7089240306", Content: "注册"}) {
		t.Fatal("second register Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, expected 2", len(messages))
	}
	if !strings.Contains(messages[0].text, "注册成功") {
		t.Fatalf("first message = %q, expected register success", messages[0].text)
	}
	if !strings.Contains(messages[1].text, "已注册，无需重复注册") {
		t.Fatalf("second message = %q, expected already registered tip", messages[1].text)
	}
}

func TestKeywordReplyVersionShowsLatestRelease(t *testing.T) {
	originalVersion := version.Version
	originalChannel := version.BuildChannel
	version.Version = "v1.0.0"
	version.BuildChannel = "local"
	defer func() {
		version.Version = originalVersion
		version.BuildChannel = originalChannel
	}()

	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	manager.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{Version: "v1.0.1", Body: "1. 修复问题"}})

	if !manager.Handle(&types.Message{Platform: "telegram", UserID: "7089240306", Content: "version"}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d", len(messages))
	}
	for _, expected := range []string{"AllBot v1.0.0 (local)", "当前版本：v1.0.0", "最新版本：v1.0.1", "更新内容：", "1. 修复问题", "发送「更新」可升级到最新版本。"} {
		if !strings.Contains(messages[0].text, expected) {
			t.Fatalf("version message missing %q: %s", expected, messages[0].text)
		}
	}
}

func TestKeywordReplyVersionAlreadyLatest(t *testing.T) {
	original := version.Version
	version.Version = "v1.0.1"
	defer func() { version.Version = original }()

	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	manager.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{Version: "v1.0.1"}})

	if !manager.Handle(&types.Message{Platform: "telegram", UserID: "7089240306", Content: "version"}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 || !strings.Contains(messages[0].text, "当前已是最新版本。") {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestKeywordReplyVersionReleaseFailure(t *testing.T) {
	original := version.Version
	version.Version = "v1.0.0"
	defer func() { version.Version = original }()

	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	manager.SetReleaseClient(fakeReleaseClient{err: errors.New("网络失败")})

	if !manager.Handle(&types.Message{Platform: "telegram", UserID: "7089240306", Content: "version"}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 || !strings.Contains(messages[0].text, "最新版本：获取失败") || !strings.Contains(messages[0].text, "网络失败") {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestKeywordReplyUpdateAdminTriggersUpgrade(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	handler := &fakeUpdateHandler{current: updater.UpgradeState{Status: updater.UpgradeStatusIdle}, start: updater.UpgradeState{Status: updater.UpgradeStatusDownloading, Message: "正在下载升级包", Version: "v1.0.2", AssetName: "allbot-windows-amd64.exe"}}
	manager.SetUpdateHandler(handler)

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "admin", Content: "更新"}) {
		t.Fatal("Handle returned false")
	}
	if handler.calls != 1 {
		t.Fatalf("upgrade calls = %d, expected 1", handler.calls)
	}
	messages := fake.sentMessages()
	if len(messages) != 1 || !strings.Contains(messages[0].text, "已开始更新到 v1.0.2") || !strings.Contains(messages[0].text, "allbot-windows-amd64.exe") {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestKeywordReplyUpdateWithoutHandlerRepliesNotInitialized(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "admin", Content: "更新"}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 || !strings.Contains(messages[0].text, "更新功能未初始化") {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestKeywordReplyUpdateNonAdminIsConsumedWithoutHandler(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, false)
	defer db.Close()
	handler := &fakeUpdateHandler{current: updater.UpgradeState{Status: updater.UpgradeStatusDownloading}}
	manager.SetUpdateHandler(handler)

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "user", Content: "更新"}) {
		t.Fatal("Handle returned false")
	}
	if handler.calls != 0 {
		t.Fatal("update handler should not be called for non-admin user")
	}
	if messages := fake.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages len = %d, expected 0", len(messages))
	}
}

func TestKeywordReplyUpdateDuplicateRequest(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	handler := &fakeUpdateHandler{current: updater.UpgradeState{Status: updater.UpgradeStatusDownloading, Message: "正在下载升级包"}}
	manager.SetUpdateHandler(handler)

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "admin", Content: "更新"}) {
		t.Fatal("Handle returned false")
	}
	if handler.calls != 0 {
		t.Fatal("update handler should not be called when upgrade is already running")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 || !strings.Contains(messages[0].text, "更新已在执行") {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestKeywordReplyRechargePointsRequiresThirdPartyPayment(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, false)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "user", Content: "积分充值 1"}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 || !strings.Contains(messages[0].text, "请先在支付配置中启用第三方支付方式") {
		t.Fatalf("unexpected messages: %#v", messages)
	}
}

func TestKeywordReplyRechargePointsCreditsAfterEpay(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mapi.php":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.PostForm.Get("type") != "alipay" || r.PostForm.Get("money") != "1.00" {
				t.Fatalf("unexpected epay form: %#v", r.PostForm)
			}
			_, _ = w.Write([]byte(`{"code":1,"trade_no":"TRECHARGE","payurl":"https://pay.example.com/recharge","qrcode":"QR-RECHARGE"}`))
		case "/api.php":
			_, _ = w.Write([]byte(`{"status":"1","trade_status":"TRADE_SUCCESS","trade_no":"TRECHARGE","type":"alipay","money":"1.00"}`))
		default:
			t.Fatalf("unexpected epay path: %s", r.URL.Path)
		}
	}))
	defer providerServer.Close()
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, false)
	defer db.Close()
	settings := config.DefaultPaymentSettings()
	settings.PointsPerRMB = 100
	settings.ThirdPartyEnabled = true
	settings.Methods = []config.PaymentMethodSetting{{Code: "points", Label: "积分支付", Provider: "points", Enabled: true}, {Code: "alipay", Label: "支付宝", Provider: "epay", Enabled: true}}
	settings.Epay = config.EpaySettings{Enabled: true, Version: "v1", APIURL: providerServer.URL + "/", PID: "1000", Key: "secret", ReturnURL: "https://app.example.com/api/open/payments/return/epay"}
	settings.EpayQueryIntervalSeconds = 1
	if err := db.SavePaymentSettings(&settings); err != nil {
		t.Fatal(err)
	}
	inputs := make(chan string, 1)
	inputs <- "alipay"
	manager.SetListenFunc(func(msg *types.Message, timeout int) string { return <-inputs })
	manager.SetListenUntilFunc(func(msg *types.Message, timeout int, done <-chan struct{}) string {
		<-done
		return ""
	})

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "user", Content: "积分充值 1"}) {
		t.Fatal("Handle returned false")
	}
	waitKeywordReplyMessages(t, fake, 3)
	account, err := db.GetUserAccount("qq", "user")
	if err != nil {
		t.Fatal(err)
	}
	balance, err := db.GetUserPoints(account.UnionID)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 100 {
		t.Fatalf("balance = %d, expected 100", balance)
	}
	messages := fake.sentMessages()
	if !containsKeywordReplyMessage(messages, "当前充值 1.00 RMB（到账 100 积分）") || !containsKeywordReplyMessage(messages, "充值成功") {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	orders, total, err := db.ListPaymentOrders(config.PaymentOrderQuery{UnionID: account.UnionID, PluginID: "builtin:recharge_points"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(orders) != 1 || orders[0].Status != "paid" {
		t.Fatalf("unexpected orders total=%d items=%#v", total, orders)
	}
	transactions, total, err := db.ListPointTransactions(config.PointTransactionQuery{UnionID: account.UnionID, Source: "recharge", SourceID: orders[0].OrderNo})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(transactions) != 1 || transactions[0].Delta != 100 || transactions[0].BalanceAfter != 100 {
		t.Fatalf("unexpected recharge transactions total=%d items=%#v", total, transactions)
	}
}

func TestSystemInfoIncludesAllBotUsage(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	info := manager.systemInfo()
	for _, expected := range []string{"系统信息", "系统：", "处理器：", "核心数：", "allBot", "内存占用：", "磁盘占用：", "%"} {
		if !strings.Contains(info, expected) {
			t.Fatalf("systemInfo missing %q: %s", expected, info)
		}
	}
}

func TestFormatSystemInfoShowsCoreThreadDescription(t *testing.T) {
	info := formatSystemInfo("Debian GNU/Linux(debian) 12.10", "CPU", "4核心4线程", "1m", "1GB", "2GB", "3MB", "4MB")
	if !strings.Contains(info, "核心数：4核心4线程") {
		t.Fatalf("system info missing core thread description: %s", info)
	}
}

func TestKeywordReplyPluginListTogglesSelectedPlugin(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	store := &fakeKeywordPluginAdminStore{plugins: []*plugincore.PluginProcess{
		{Plugin: &types.Plugin{ID: "demo", Name: "测试插件", Enabled: false}},
	}}
	inputs := make(chan string, 3)
	inputs <- "1"
	inputs <- "1"
	inputs <- "q"
	manager.SetPluginAdminStore(store)
	manager.SetListenFunc(func(msg *types.Message, timeout int) string { return <-inputs })

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "admin", Content: "插件列表"}) {
		t.Fatal("Handle returned false")
	}
	waitKeywordReplyMessages(t, fake, 4)
	if !store.plugins[0].Plugin.Enabled {
		t.Fatal("plugin should be enabled")
	}
	messages := fake.sentMessages()
	if !strings.Contains(messages[0].text, "1. 测试插件(demo) ❌") {
		t.Fatalf("list message missing disabled plugin: %s", messages[0].text)
	}
	if !containsKeywordReplyMessage(messages, "已启动【测试插件】") {
		t.Fatalf("toggle confirmation missing: %#v", messages)
	}
}

func TestKeywordReplyPluginListUpdatesAccessControlIncrementally(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	store := &fakeKeywordPluginAdminStore{plugins: []*plugincore.PluginProcess{
		{Plugin: &types.Plugin{ID: "demo", Name: "测试插件", Enabled: true, AccessControl: types.AccessControlConfig{InheritSystem: true, WhitelistGroups: []string{"old"}, BlockedUserIDs: []string{"blocked"}}}},
	}}
	inputs := make(chan string, 5)
	inputs <- "1"
	inputs <- "2"
	inputs <- "1"
	inputs <- "+123,+456,-old"
	inputs <- "q"
	manager.SetPluginAdminStore(store)
	manager.SetListenFunc(func(msg *types.Message, timeout int) string { return <-inputs })

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "admin", Content: "插件列表"}) {
		t.Fatal("Handle returned false")
	}
	waitKeywordReplyMessages(t, fake, 5)
	accessControl := store.plugins[0].Plugin.AccessControl
	if strings.Join(accessControl.WhitelistGroups, ",") != "123,456" {
		t.Fatalf("WhitelistGroups = %#v", accessControl.WhitelistGroups)
	}
	if strings.Join(accessControl.BlockedUserIDs, ",") != "blocked" {
		t.Fatalf("BlockedUserIDs should be preserved: %#v", accessControl.BlockedUserIDs)
	}
	if accessControl.InheritSystem {
		t.Fatal("plugin-specific access rules should disable system inheritance")
	}
	if !containsKeywordReplyMessage(fake.sentMessages(), "当前值：123,456") {
		t.Fatalf("update confirmation missing: %#v", fake.sentMessages())
	}
}

func TestKeywordReplyPluginListUpdatesUnionIDAccessControl(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	store := &fakeKeywordPluginAdminStore{plugins: []*plugincore.PluginProcess{
		{Plugin: &types.Plugin{ID: "demo", Name: "测试插件", Enabled: true}},
	}}
	inputs := make(chan string, 5)
	inputs <- "1"
	inputs <- "2"
	inputs <- "5"
	inputs <- "+union-1,+union-2"
	inputs <- "q"
	manager.SetPluginAdminStore(store)
	manager.SetListenFunc(func(msg *types.Message, timeout int) string { return <-inputs })

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "admin", Content: "插件列表"}) {
		t.Fatal("Handle returned false")
	}
	waitKeywordReplyMessages(t, fake, 5)
	if strings.Join(store.plugins[0].Plugin.AccessControl.WhitelistUnionIDs, ",") != "union-1,union-2" {
		t.Fatalf("WhitelistUnionIDs = %#v", store.plugins[0].Plugin.AccessControl.WhitelistUnionIDs)
	}
}

func TestKeywordReplyPluginListPaginatesPlugins(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	store := &fakeKeywordPluginAdminStore{}
	for i := 1; i <= 11; i++ {
		store.plugins = append(store.plugins, &plugincore.PluginProcess{Plugin: &types.Plugin{ID: fmt.Sprintf("p%02d", i), Name: fmt.Sprintf("插件%02d", i), Enabled: true, Order: i}})
	}
	inputs := make(chan string, 2)
	inputs <- "下一页"
	inputs <- "q"
	manager.SetPluginAdminStore(store)
	manager.SetListenFunc(func(msg *types.Message, timeout int) string { return <-inputs })

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "admin", Content: "插件列表"}) {
		t.Fatal("Handle returned false")
	}
	waitKeywordReplyMessages(t, fake, 3)
	messages := fake.sentMessages()
	if !strings.Contains(messages[0].text, "插件列表 第1/2页") || !strings.Contains(messages[1].text, "插件列表 第2/2页") {
		t.Fatalf("pagination messages unexpected: %#v", messages)
	}
	if !strings.Contains(messages[1].text, "1. 插件11(p11) ✅") {
		t.Fatalf("second page missing plugin 11: %s", messages[1].text)
	}
}

func waitKeywordReplyMessages(t *testing.T, fake *keywordReplyFakeAdapter, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(fake.sentMessages()) >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("messages len = %d, expected at least %d", len(fake.sentMessages()), count)
}

func containsKeywordReplyMessage(messages []sentKeywordReplyMessage, expected string) bool {
	for _, message := range messages {
		if strings.Contains(message.text, expected) {
			return true
		}
	}
	return false
}

func TestKeywordReplyRestartAdminTriggersHandler(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	restarted := make(chan RestartRequest, 1)
	manager.SetRestartHandler(func(request RestartRequest) error {
		restarted <- request
		return nil
	})

	msg := &types.Message{ID: "m1", Platform: "qq", AdapterID: "7", UserID: "1001", Content: "重启"}
	handled := manager.Handle(msg)
	if !handled {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if messages[0].target != "1001" {
		t.Fatalf("target = %q, expected 1001", messages[0].target)
	}
	if !strings.Contains(messages[0].text, "AllBot 正在重启") {
		t.Fatalf("message = %q, expected restart confirmation", messages[0].text)
	}

	select {
	case request := <-restarted:
		if request.MessageKey != RestartMessageKey(msg) {
			t.Fatal("restart request should include source message key")
		}
		if request.AdapterID != "7" || request.Target != "1001" || request.UserID != "1001" {
			t.Fatalf("unexpected restart request: %+v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("restart handler was not called")
	}
}

func TestKeywordReplyRestartIgnoresSourceMessageAfterProcessRestart(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	msg := &types.Message{ID: "same-message", Platform: "telegram", AdapterID: "3", UserID: "1001", GroupID: "2001", Content: "重启"}
	t.Setenv("ALLBOT_IGNORE_RESTART_MESSAGE_KEY", RestartMessageKey(msg))
	called := false
	manager.SetRestartHandler(func(request RestartRequest) error {
		called = true
		return nil
	})

	handled := manager.Handle(msg)
	if !handled {
		t.Fatal("Handle returned false")
	}
	if called {
		t.Fatal("restart handler should not be called for ignored restart message")
	}
	if messages := fake.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages len = %d, expected 0", len(messages))
	}
}

func TestKeywordReplyRestartWithoutHandlerRepliesNotInitialized(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	handled := manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"})
	if !handled {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if !strings.Contains(messages[0].text, "重启功能未初始化") {
		t.Fatalf("message = %q, expected initialization failure", messages[0].text)
	}
}

func TestKeywordReplyRestartNonAdminIsConsumedWithoutHandler(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, false)
	defer db.Close()

	called := false
	manager.SetRestartHandler(func(request RestartRequest) error {
		called = true
		return nil
	})

	handled := manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"})
	if !handled {
		t.Fatal("Handle returned false")
	}
	if called {
		t.Fatal("restart handler should not be called for non-admin user")
	}
	if messages := fake.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages len = %d, expected 0", len(messages))
	}
}

func TestKeywordReplyRestartDuplicateRequest(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	block := make(chan struct{})
	manager.SetRestartHandler(func(request RestartRequest) error {
		<-block
		return nil
	})
	defer close(block)

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("first Handle returned false")
	}
	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("second Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, expected 2", len(messages))
	}
	if !strings.Contains(messages[1].text, "重启已在执行") {
		t.Fatalf("message = %q, expected duplicate restart warning", messages[1].text)
	}
}

func TestKeywordReplyRestartHandlerFailureReleasesRequest(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	calls := 0
	done := make(chan struct{}, 2)
	manager.SetRestartHandler(func(request RestartRequest) error {
		calls++
		done <- struct{}{}
		return errors.New("重启失败")
	})

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("first Handle returned false")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first restart handler was not called")
	}
	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("second Handle returned false")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second restart handler was not called")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, expected 2", calls)
	}
}

func TestKeywordReplyQQOfficeUsesReplyTarget(t *testing.T) {
	qqOffice := qqofficeadapter.NewQQOfficeAdapter("app123", "secret456", "", "")
	fake := newReplyCapableKeywordReplyFakeAdapter(qqOffice, qqOffice)
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "user123", Content: "version", Metadata: map[string]string{"reply_target": "dms_guild123|msg_msg456"}}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if messages[0].target != "dms_guild123|msg_msg456" {
		t.Fatalf("target = %q", messages[0].target)
	}
}

func TestKeywordReplyQQOfficeUsesGuildIDFallback(t *testing.T) {
	qqOffice := qqofficeadapter.NewQQOfficeAdapter("app123", "secret456", "", "")
	fake := newReplyCapableKeywordReplyFakeAdapter(qqOffice, qqOffice)
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "user123", Content: "version", Metadata: map[string]string{"qq_office_guild_id": "guild123"}}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if messages[0].target != "dms_guild123" {
		t.Fatalf("target = %q", messages[0].target)
	}
	if strings.Contains(messages[0].text, "[CQ:at") {
		t.Fatalf("message should not contain CQ at: %q", messages[0].text)
	}
}

func TestKeywordReplyQQOfficeUsesOpenIDFallbacks(t *testing.T) {
	qqOffice := qqofficeadapter.NewQQOfficeAdapter("app123", "secret456", "", "")
	fake := newReplyCapableKeywordReplyFakeAdapter(qqOffice, qqOffice)
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "user-openid", Content: "version", Metadata: map[string]string{"qq_office_user_openid": "user-openid"}}) {
		t.Fatal("Handle C2C returned false")
	}
	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "member-openid", GroupID: "group-openid", Content: "version", Metadata: map[string]string{"qq_office_group_openid": "group-openid"}}) {
		t.Fatal("Handle group returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, expected 2", len(messages))
	}
	if messages[0].target != "user_user-openid" {
		t.Fatalf("first target = %q", messages[0].target)
	}
	if messages[1].target != "group_group-openid" {
		t.Fatalf("second target = %q", messages[1].target)
	}
	if strings.Contains(messages[1].text, "@member-openid") {
		t.Fatalf("QQ official group reply should not add text mention: %q", messages[1].text)
	}
}
