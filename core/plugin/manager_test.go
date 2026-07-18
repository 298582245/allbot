package plugin

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/deps"
	"github.com/allbot/allbot/core/types"
)

func TestSetPluginPinnedPersistsConfigAndMemory(t *testing.T) {
	pluginRoot := t.TempDir()
	pluginDir := filepath.Join(pluginRoot, "demo")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "entry.js"), []byte("console.log('ok')"), 0644); err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{
		"name":         "演示插件",
		"version":      "1.0.0",
		"runtime":      "nodejs",
		"entry":        "entry.js",
		"platforms":    []string{"qq"},
		"priority":     0,
		"trigger":      "^demo$",
		"enabled":      true,
		"dependencies": map[string]string{},
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(pluginRoot, deps.NewManager(t.TempDir()))
	if _, err := manager.LoadPlugin(pluginDir); err != nil {
		t.Fatalf("LoadPlugin returned error: %v", err)
	}
	if err := manager.SetPluginPinned("demo", true); err != nil {
		t.Fatalf("SetPluginPinned returned error: %v", err)
	}
	process := manager.GetPlugin("demo")
	if process == nil || process.Plugin == nil || !process.Plugin.Pinned {
		t.Fatalf("plugin should be pinned in memory: %#v", process)
	}
	stored, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(stored, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["pinned"] != true {
		t.Fatalf("plugin.json should persist pinned=true, got %#v", raw["pinned"])
	}
}

func TestExecutePluginPaymentWaitWritesResponse(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'payment_wait', request_id: 'pay-1', subject: '测试支付', amount: '9.90', timeout: 12, union_id: 'union-action', methods: ['alipay'], metadata: { k: 'v' }, remark: '订单备注' }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  const ok = response.action === 'payment_response' && response.request_id === 'pay-1' && response.success === true && response.data.order_no === 'P1';
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	var received PaymentWaitAction
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{"content":"/pay"}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action PaymentWaitAction) PluginUserResult {
		received = action
		return PluginUserResult{Success: true, Data: map[string]interface{}{"order_no": "P1"}}
	})
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if received.RequestID != "pay-1" || received.Subject != "测试支付" || string(received.AmountRaw) != `"9.90"` || received.Timeout != 12 || received.UnionID != "union-action" || len(received.Methods) != 1 || received.Methods[0] != "alipay" || received.Remark != "订单备注" {
		t.Fatalf("unexpected payment action: %#v amount=%s", received, string(received.AmountRaw))
	}
}

func TestExecutePluginPaymentWaitWritesFailureResponse(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'payment_wait', request_id: 'pay-fail', subject: '失败支付', amount: '1.00' }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  const ok = response.action === 'payment_response' && response.request_id === 'pay-fail' && response.success === false && response.error === '支付失败';
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action PaymentWaitAction) PluginUserResult {
		return PluginUserResult{Success: false, Error: "支付失败", Data: map[string]interface{}{"status": "failed"}}
	})
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
}

func TestExecutePluginPointActionsStillParseAmounts(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'points_consume', request_id: 'c1', union_id: 'u1', amount: 7 }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  if (step === 1) {
    step = 2;
    if (response.action !== 'auth_response' || response.request_id !== 'c1' || !response.success) {
      process.stdout.write(JSON.stringify({ action: 'done', success: false, error: JSON.stringify(response) }) + '\n');
      return;
    }
    process.stdout.write(JSON.stringify({ action: 'points_add', request_id: 'a1', union_id: 'u1', amount: '8' }) + '\n');
    return;
  }
  const ok = response.action === 'auth_response' && response.request_id === 'a1' && response.success;
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	amounts := []int64{}
	actions := []string{}
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action PluginAuthorizationAction) PluginUserResult {
		actions = append(actions, action.Action)
		amounts = append(amounts, action.Amount)
		return PluginUserResult{Success: true, Data: map[string]interface{}{"points": 100}}
	}, nil, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if len(actions) != 2 || actions[0] != "points_consume" || actions[1] != "points_add" || amounts[0] != 7 || amounts[1] != 8 {
		t.Fatalf("unexpected auth actions=%#v amounts=%#v", actions, amounts)
	}
}

func TestExecutePluginSendButtonsUsesFallbacks(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
rl.on('line', () => {
  process.stdout.write(JSON.stringify({ action: 'send_buttons', text: '请选择', buttons: [[{ text: 'A', value: '1', user_id: 'u1' }]] }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'send_buttons', content: '内容', buttons: [[{ text: 'B', value: '2' }]] }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'send_buttons', markdown: '标题', buttons: [[{ text: 'C', value: '3' }]] }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'done', success: true }) + '\n');
});
`)
	received := make([]struct {
		text    string
		buttons [][]types.ButtonOption
	}, 0, 3)
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(text string, buttons [][]types.ButtonOption) error {
		received = append(received, struct {
			text    string
			buttons [][]types.ButtonOption
		}{text: text, buttons: buttons})
		return nil
	})
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if len(received) != 3 || received[0].text != "请选择" || received[1].text != "内容" || received[2].text != "标题" {
		t.Fatalf("unexpected button replies: %#v", received)
	}
	if len(received[0].buttons) != 1 || received[0].buttons[0][0].Text != "A" || received[0].buttons[0][0].Value != "1" || received[0].buttons[0][0].UserID != "u1" {
		t.Fatalf("unexpected first buttons: %#v", received[0].buttons)
	}
}

func TestExecutePluginSendMarkdownUsesMarkdownFallbacks(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
rl.on('line', () => {
  process.stdout.write(JSON.stringify({ action: 'send_markdown', markdown: '## 标题' }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'send_markdown', content: '**内容**' }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'send_markdown', text: '*文本*' }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'done', success: true }) + '\n');
});
`)
	received := []string{}
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(markdown string) error {
		received = append(received, markdown)
		return nil
	})
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if len(received) != 3 || received[0] != "## 标题" || received[1] != "**内容**" || received[2] != "*文本*" {
		t.Fatalf("unexpected markdown replies: %#v", received)
	}
}

func TestExecutePluginSendRichUsesCallback(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
rl.on('line', () => {
  process.stdout.write(JSON.stringify({ action: 'send_rich', parts: [{ type: 'text', text: '商品' }, { type: 'image', url: 'https://example.com/a.png', alt: '图' }], fallback_text: '商品 图', prefer: 'auto' }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'done', success: true }) + '\n');
});
`)
	var received types.RichMessage
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(message types.RichMessage) error {
		received = message
		return nil
	})
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if received.FallbackText != "商品 图" || received.Prefer != "auto" || len(received.Parts) != 2 || received.Parts[1].URL != "https://example.com/a.png" {
		t.Fatalf("unexpected rich message: %#v", received)
	}
}

func TestExecutePluginSendRichMessageWritesResponse(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'send_rich_message', request_id: 'rich-1', platform: 'telegram', user_id: 'u1', parts: [{ type: 'markdown', markdown: '**中文**' }], prefer: 'markdown' }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  const ok = response.action === 'send_rich_message_response' && response.request_id === 'rich-1' && response.success === true;
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	var received RichMessageAction
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action RichMessageAction) PluginUserResult {
		received = action
		return PluginUserResult{Success: true, Data: true}
	})
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if received.Platform != "telegram" || received.UserID != "u1" || received.Prefer != "markdown" || len(received.Parts) != 1 || received.Parts[0].Markdown != "**中文**" {
		t.Fatalf("unexpected rich action: %#v", received)
	}
}

func TestExecutePluginSendImageMessageWritesResponseAndKeepsLegacySendImage(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'send_image', url: 'https://example.com/legacy.png' }) + '\n');
    process.stdout.write(JSON.stringify({ action: 'send_image_message', request_id: 'image-1', platform: 'qq_office', adapter_id: '7', group_id: 'group-1', union_id: 'union-1', url: 'https://example.com/image.png' }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  const ok = response.action === 'send_image_message_response' && response.request_id === 'image-1' && response.success === true && response.data.sent === true;
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	legacyURL := ""
	var received ImageMessageAction
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, func(url string) error {
		legacyURL = url
		return nil
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action ImageMessageAction) PluginUserResult {
		received = action
		return PluginUserResult{Success: true, Data: map[string]interface{}{"sent": true}}
	})
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if legacyURL != "https://example.com/legacy.png" {
		t.Fatalf("legacy send_image should remain unchanged, got %q", legacyURL)
	}
	if received.Platform != "qq_office" || received.AdapterID != "7" || received.GroupID != "group-1" || received.UnionID != "union-1" || received.URL != "https://example.com/image.png" {
		t.Fatalf("unexpected image action: %#v", received)
	}
}

func TestExecutePluginSendMessageUsesContentFallback(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'send_message', request_id: 'send-1', platform: 'telegram', user_id: 'u1', content: 'fallback text' }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  const ok = response.action === 'send_message_response' && response.request_id === 'send-1' && response.success === true;
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	var received SendMessageAction
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action SendMessageAction) PluginUserResult {
		received = action
		return PluginUserResult{Success: true, Data: true}
	}, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if received.Platform != "telegram" || received.UserID != "u1" || received.Text != "fallback text" {
		t.Fatalf("unexpected send message action: %#v", received)
	}
}

func TestExecutePluginStdoutDebugLineKeepsProtocol(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
rl.on('line', () => {
  process.stdout.write('debug before action\n');
  process.stdout.write(JSON.stringify({ action: 'reply', text: 'ok' }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'done', success: true }) + '\n');
});
`)
	logs := captureStandardLog(t)
	replied := ""
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), func(text string) error {
		replied = text
		return nil
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if replied != "ok" {
		t.Fatalf("expected reply action to continue after debug line, got %q", replied)
	}
	if !strings.Contains(logs.String(), "[SYSTEM][PLUGIN][plugin-test][STDOUT] debug before action") {
		t.Fatalf("expected stdout debug log, got %q", logs.String())
	}
}

func TestExecutePluginStderrMergesBurstLines(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
rl.on('line', () => {
  process.stderr.write('stderr one\n');
  process.stderr.write('stderr two\n');
  process.stdout.write(JSON.stringify({ action: 'done', success: true }) + '\n');
});
`)
	logs := captureStandardLog(t)
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	output := logs.String()
	if !strings.Contains(output, "[SYSTEM][PLUGIN][plugin-test][STDERR] stderr one\nstderr two") {
		t.Fatalf("expected merged stderr log, got %q", output)
	}
	if strings.Count(output, "[SYSTEM][PLUGIN][plugin-test][STDERR]") != 1 {
		t.Fatalf("expected one stderr log entry, got %q", output)
	}
}

func TestExecutePluginJSONWithoutActionLogsStdout(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
rl.on('line', () => {
  process.stdout.write(JSON.stringify({ message: 'missing action' }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'unknown_action', text: 'debug json' }) + '\n');
  process.stdout.write(JSON.stringify({ action: 'done', success: true }) + '\n');
});
`)
	logs := captureStandardLog(t)
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	output := logs.String()
	if !strings.Contains(output, "[SYSTEM][PLUGIN][plugin-test][STDOUT] {\"message\":\"missing action\"}\n{\"action\":\"unknown_action\",\"text\":\"debug json\"}") {
		t.Fatalf("expected merged missing/unknown action stdout log, got %q", output)
	}
	if strings.Count(output, "[SYSTEM][PLUGIN][plugin-test][STDOUT]") != 1 {
		t.Fatalf("expected one stdout log entry, got %q", output)
	}
}

func TestExecutePluginWebSendRichAndImageMessagesWriteResponses(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'send_rich_message', request_id: 'rich-web', platform: 'telegram', user_id: 'u1', parts: [{ type: 'markdown', markdown: '**Web**' }], fallback_text: 'Web' }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  if (step === 1) {
    if (response.action !== 'send_rich_message_response' || !response.success) process.exit(2);
    step = 2;
    process.stdout.write(JSON.stringify({ action: 'send_image_message', request_id: 'image-web', platform: 'telegram', user_id: 'u1', url: 'https://example.com/web.png' }) + '\n');
    return;
  }
  const ok = response.action === 'send_image_message_response' && response.success;
  process.stdout.write(JSON.stringify({ action: 'web_response', status: 200, json: { ok } }) + '\n');
});
`)
	var rich RichMessageAction
	var image ImageMessageAction
	response, err := manager.ExecutePluginWeb(plugin, pluginPath, []byte(`{"event_type":"web_api"}`), nil, nil, OpenAPIExecutors{
		SendRichMessage: func(pluginID string, action RichMessageAction) PluginUserResult {
			rich = action
			return PluginUserResult{Success: true, Data: true}
		},
		SendImage: func(pluginID string, action ImageMessageAction) PluginUserResult {
			image = action
			return PluginUserResult{Success: true, Data: true}
		},
	})
	if err != nil {
		t.Fatalf("ExecutePluginWeb returned error: %v", err)
	}
	if rich.UserID != "u1" || len(rich.Parts) != 1 || rich.FallbackText != "Web" {
		t.Fatalf("unexpected rich action: %#v", rich)
	}
	if image.UserID != "u1" || image.URL != "https://example.com/web.png" {
		t.Fatalf("unexpected image action: %#v", image)
	}
	responseJSON, ok := response.JSON.(map[string]interface{})
	if !ok || responseJSON["ok"] != true {
		t.Fatalf("unexpected web response: %#v", response)
	}
}

func TestExecuteOpenAPISendRichAndImageMessagesWriteResponses(t *testing.T) {
	manager, endpoint, workDir := newManagerTestOpenAPI(t, `
exports.action = async (ctx, req, res) => {
  const rich = await ctx.sendRichMessage({ platform: 'telegram', adapterId: '5', userId: 'user-1', unionId: 'union-1', parts: [{ type: 'markdown', markdown: '**通知**' }], fallbackText: '通知', prefer: 'markdown' });
  const image = await ctx.sendImageMessage({ platform: 'qq_office', adapterId: '7', groupId: 'group-1', unionId: 'union-2', url: 'https://example.com/image.png' });
  res.json({ rich, image });
};
`)
	var receivedRich RichMessageAction
	var receivedImage ImageMessageAction
	response, err := manager.ExecuteOpenAPI(endpoint, workDir, types.OpenAPIRequest{}, nil, nil, OpenAPIExecutors{
		SendRichMessage: func(openAPIID string, action RichMessageAction) PluginUserResult {
			receivedRich = action
			return PluginUserResult{Success: true, Data: map[string]interface{}{"kind": "rich"}}
		},
		SendImage: func(openAPIID string, action ImageMessageAction) PluginUserResult {
			receivedImage = action
			return PluginUserResult{Success: true, Data: map[string]interface{}{"kind": "image"}}
		},
	})
	if err != nil {
		t.Fatalf("ExecuteOpenAPI returned error: %v", err)
	}
	if receivedRich.Platform != "telegram" || receivedRich.AdapterID != "5" || receivedRich.UserID != "user-1" || receivedRich.UnionID != "union-1" || receivedRich.FallbackText != "通知" || receivedRich.Prefer != "markdown" || len(receivedRich.Parts) != 1 || receivedRich.Parts[0].Markdown != "**通知**" {
		t.Fatalf("unexpected rich action: %#v", receivedRich)
	}
	if receivedImage.Platform != "qq_office" || receivedImage.AdapterID != "7" || receivedImage.GroupID != "group-1" || receivedImage.UnionID != "union-2" || receivedImage.URL != "https://example.com/image.png" {
		t.Fatalf("unexpected image action: %#v", receivedImage)
	}
	responseJSON, ok := response.JSON.(map[string]interface{})
	if !ok || responseJSON["rich"] == nil || responseJSON["image"] == nil {
		t.Fatalf("unexpected OpenAPI response: %#v", response)
	}
}

func TestExecuteOpenAPIStdoutDebugLineKeepsHTTPResponse(t *testing.T) {
	manager, endpoint, workDir := newManagerTestOpenAPI(t, `
exports.action = (ctx, req, res) => {
  process.stdout.write('openapi debug\n');
  res.json({ ok: true });
};
`)
	logs := captureStandardLog(t)
	response, err := manager.ExecuteOpenAPI(endpoint, workDir, types.OpenAPIRequest{}, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteOpenAPI returned error: %v", err)
	}
	if response.Status != 200 || response.JSON == nil {
		t.Fatalf("unexpected response: %#v", response)
	}
	if !strings.Contains(logs.String(), "[SYSTEM][OPENAPI][openapi-test][STDOUT] openapi debug") {
		t.Fatalf("expected openapi stdout debug log, got %q", logs.String())
	}
}

func TestExecuteOpenAPIStderrMergesBurstLines(t *testing.T) {
	manager, endpoint, workDir := newManagerTestOpenAPI(t, `
exports.action = (ctx, req, res) => {
  process.stderr.write('openapi stderr one\n');
  process.stderr.write('openapi stderr two\n');
  res.send('ok');
};
`)
	logs := captureStandardLog(t)
	_, err := manager.ExecuteOpenAPI(endpoint, workDir, types.OpenAPIRequest{}, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteOpenAPI returned error: %v", err)
	}
	output := logs.String()
	if !strings.Contains(output, "[SYSTEM][OPENAPI][openapi-test][STDERR] openapi stderr one\nopenapi stderr two") {
		t.Fatalf("expected merged openapi stderr log, got %q", output)
	}
	if strings.Count(output, "[SYSTEM][OPENAPI][openapi-test][STDERR]") != 1 {
		t.Fatalf("expected one openapi stderr log entry, got %q", output)
	}
}

func captureStandardLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})
	return &logs
}

func newManagerTestPlugin(t *testing.T, script string) (*Manager, *types.Plugin, string) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不可用，跳过 Direct 插件协议测试")
	}
	pluginPath := t.TempDir()
	entry := filepath.Join(pluginPath, "entry.js")
	if err := os.WriteFile(entry, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	plugin := &types.Plugin{ID: "plugin-test", Name: "测试插件", Runtime: "nodejs", Entry: "entry.js", Enabled: true}
	return manager, plugin, pluginPath
}

func newManagerTestOpenAPI(t *testing.T, script string) (*Manager, types.OpenAPIEndpoint, string) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不可用，跳过 OpenAPI 插件协议测试")
	}
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(previousDir, "..", ".."))
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})
	workDir := t.TempDir()
	entry := filepath.Join(workDir, "entry.js")
	if err := os.WriteFile(entry, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(workDir, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	endpoint := types.OpenAPIEndpoint{ID: "openapi-test", Name: "测试 OpenAPI", Runtime: "nodejs", Entry: "entry.js", Enabled: true}
	return manager, endpoint, workDir
}

func TestExecutePluginRunScriptPassesExplicitRuntimeProfile(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'run_script', request_id: 'script-1', runtime: 'nodejs', runtime_profile: 'node18', script: 'task.js', wait: true }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  const ok = response.action === 'script_response' && response.request_id === 'script-1' && response.success === true;
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	var received ScriptRunAction
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action ScriptRunAction) PluginUserResult {
		received = action
		return PluginUserResult{Success: true, Data: map[string]interface{}{"ok": true}}
	}, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if received.PluginID != plugin.ID || received.RuntimeProfile != "node18" || received.Script != "task.js" || !received.Wait {
		t.Fatalf("unexpected script action: %#v", received)
	}
}

func TestExecutePluginRunScriptInheritsPluginRuntimeProfile(t *testing.T) {
	manager, plugin, pluginPath := newManagerTestPlugin(t, `
const readline = require('readline');
const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
let step = 0;
rl.on('line', (line) => {
  if (step === 0) {
    step = 1;
    process.stdout.write(JSON.stringify({ action: 'run_script', request_id: 'script-2', runtime: 'nodejs', script: 'task.js' }) + '\n');
    return;
  }
  const response = JSON.parse(line);
  const ok = response.action === 'script_response' && response.request_id === 'script-2' && response.success === true;
  process.stdout.write(JSON.stringify({ action: 'done', success: ok, error: ok ? '' : JSON.stringify(response) }) + '\n');
});
`)
	plugin.RuntimeProfile = "node18"
	var received ScriptRunAction
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action ScriptRunAction) PluginUserResult {
		received = action
		return PluginUserResult{Success: true, Data: map[string]interface{}{"ok": true}}
	}, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if received.RuntimeProfile != "node18" {
		t.Fatalf("expected inherited profile, got %#v", received)
	}
}

func TestExecutePluginRunScriptDoesNotInheritDifferentRuntimeProfile(t *testing.T) {
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("python 不可用，跳过 Python Direct 插件协议测试")
	}
	pluginPath := t.TempDir()
	script := `import json
import sys

step = 0
for line in sys.stdin:
    if step == 0:
        step = 1
        print(json.dumps({"action": "run_script", "request_id": "script-mixed", "runtime": "nodejs", "script": "task.js"}), flush=True)
        continue
    response = json.loads(line)
    ok = response.get("action") == "script_response" and response.get("request_id") == "script-mixed" and response.get("success") is True
    print(json.dumps({"action": "done", "success": ok, "error": "" if ok else json.dumps(response)}), flush=True)
`
	if err := os.WriteFile(filepath.Join(pluginPath, "entry.py"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	plugin := &types.Plugin{ID: "plugin-test", Name: "测试插件", Runtime: "python", RuntimeProfile: "python-default", Entry: "entry.py", Enabled: true}
	var received ScriptRunAction
	err := manager.ExecutePlugin(plugin, pluginPath, []byte(`{}`), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(pluginID string, action ScriptRunAction) PluginUserResult {
		received = action
		return PluginUserResult{Success: true, Data: map[string]interface{}{"ok": true}}
	}, nil)
	if err != nil {
		t.Fatalf("ExecutePlugin returned error: %v", err)
	}
	if received.Runtime != "nodejs" || received.RuntimeProfile != "" {
		t.Fatalf("expected nodejs script without inherited python profile, got %#v", received)
	}
}

func TestRunPluginScriptRecordsRuntimeProfileAndEnv(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不可用，跳过脚本执行测试")
	}
	pluginPath := t.TempDir()
	script := `console.log(JSON.stringify({ profile: process.env.ALLBOT_RUNTIME_PROFILE, nodePath: process.env.NODE_PATH || '' }));`
	if err := os.WriteFile(filepath.Join(pluginPath, "task.js"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager.SetDatabase(database)

	result := manager.RunPluginScript(pluginPath, ScriptRunAction{PluginID: "plugin-test", Runtime: "nodejs", RuntimeProfile: "node18", Script: "task.js", Wait: true, Timeout: 10})
	if !result.Success {
		t.Fatalf("RunPluginScript failed: %#v", result)
	}
	items, err := database.ListScriptRunLogs(config.ScriptRunLogFilter{PluginID: "plugin-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RuntimeProfile != "node18" {
		t.Fatalf("unexpected logs: %#v", items)
	}
	detail, err := database.GetScriptRunLog(items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	output := detail.Output
	if !strings.Contains(output, "node18") {
		t.Fatalf("expected output to include runtime profile env, got %q", output)
	}
	if !strings.Contains(output, "node_modules") {
		t.Fatalf("expected output to include NODE_PATH, got %q", output)
	}
}

func TestRunPluginScriptReusesSameScriptTaskRecord(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不可用，跳过脚本执行测试")
	}
	pluginPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginPath, "task.js"), []byte("console.log('ok')\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager.SetDatabase(database)

	action := ScriptRunAction{PluginID: "plugin-test", Runtime: "nodejs", RuntimeProfile: "node18", Script: "task.js", RunMode: "current_user", UnionID: "union-a", Wait: true, Timeout: 10}
	first := manager.RunPluginScript(pluginPath, action)
	if !first.Success {
		t.Fatalf("first RunPluginScript failed: %#v", first)
	}
	second := manager.RunPluginScript(pluginPath, action)
	if !second.Success {
		t.Fatalf("second RunPluginScript failed: %#v", second)
	}
	items, err := database.ListScriptRunLogs(config.ScriptRunLogFilter{PluginID: "plugin-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RunTotal != 2 {
		t.Fatalf("expected one reused script task with run_total=2, got %#v", items)
	}
}

func TestRunPluginScriptInjectsEnabledScriptEnv(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不可用，跳过脚本执行测试")
	}
	pluginPath := t.TempDir()
	script := `console.log(JSON.stringify({ token: process.env.API_TOKEN || '', explicit: process.env.EXPLICIT || '', disabled: process.env.DISABLED || '' }));`
	if err := os.WriteFile(filepath.Join(pluginPath, "task.js"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager.SetDatabase(database)
	if _, err := database.SaveScriptEnvVar(&config.ScriptEnvVar{Name: "API_TOKEN", Value: "secret", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SaveScriptEnvVar(&config.ScriptEnvVar{Name: "API_TOKEN", Value: "second", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SaveScriptEnvVar(&config.ScriptEnvVar{Name: "DISABLED", Value: "skip", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	manager.plugins["plugin-test"] = &PluginProcess{Plugin: &types.Plugin{ID: "plugin-test", ScriptEnv: types.ScriptEnvConfig{Enabled: true, Names: []string{"API_TOKEN", "DISABLED"}}}}

	result := manager.RunPluginScript(pluginPath, ScriptRunAction{PluginID: "plugin-test", Runtime: "nodejs", RuntimeProfile: "node18", Script: "task.js", Env: map[string]string{"EXPLICIT": "value"}, Wait: true, Timeout: 10})
	if !result.Success {
		t.Fatalf("RunPluginScript failed: %#v", result)
	}
	items, err := database.ListScriptRunLogs(config.ScriptRunLogFilter{PluginID: "plugin-test"})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := database.GetScriptRunLog(items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail.Output, `"token":"second&secret"`) || !strings.Contains(detail.Output, `"explicit":"value"`) || strings.Contains(detail.Output, "skip") {
		t.Fatalf("unexpected output: %q", detail.Output)
	}
}

func TestRunPluginScriptUsesUnbufferedPython(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python 不可用，跳过脚本执行测试")
	}
	pluginPath := t.TempDir()
	script := "import os\nprint('unbuffered=' + os.getenv('PYTHONUNBUFFERED', ''))\n"
	if err := os.WriteFile(filepath.Join(pluginPath, "task.py"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	_, err = manager.GetDepsManager().SaveRuntimeProfiles([]deps.RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: pythonPath, Enabled: true, Default: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager.SetDatabase(database)

	result := manager.RunPluginScript(pluginPath, ScriptRunAction{PluginID: "plugin-python", Runtime: "python", Script: "task.py", Wait: true, Timeout: 10})
	if !result.Success {
		t.Fatalf("RunPluginScript failed: %#v", result)
	}
	items, err := database.ListScriptRunLogs(config.ScriptRunLogFilter{PluginID: "plugin-python"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected logs: %#v", items)
	}
	detail, err := database.GetScriptRunLog(items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail.Output, "unbuffered=1") {
		t.Fatalf("expected unbuffered python output, got %q", detail.Output)
	}
}

func TestRunPluginScriptRejectsMissingRuntimeProfile(t *testing.T) {
	pluginPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(pluginPath, "task.js"), []byte("console.log('ok')\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager.SetDatabase(database)

	result := manager.RunPluginScript(pluginPath, ScriptRunAction{PluginID: "plugin-test", Runtime: "nodejs", RuntimeProfile: "missing", Script: "task.js", Wait: true})
	if result.Success || !strings.Contains(result.Error, "运行环境 Profile 不存在或未启用") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunPluginScriptStopsAfterRunTimeout(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不可用，跳过脚本执行测试")
	}
	pluginPath := t.TempDir()
	script := "setInterval(() => console.log('tick'), 100)\n"
	if err := os.WriteFile(filepath.Join(pluginPath, "task.js"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.SaveScriptTaskSettings(config.ScriptTaskSettings{RunTimeoutSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	manager.SetDatabase(database)
	notifications := make(chan ScriptTimeoutNotification, 1)
	manager.SetScriptTimeoutNotifier(func(notification ScriptTimeoutNotification) { notifications <- notification })

	result := manager.RunPluginScript(pluginPath, ScriptRunAction{PluginID: "plugin-timeout", Runtime: "nodejs", RuntimeProfile: "node18", Script: "task.js", Wait: true, Timeout: 5})
	if result.Success || !strings.Contains(result.Error, "脚本任务运行超过 1 秒，已自动停止") {
		t.Fatalf("unexpected timeout result: %#v", result)
	}
	items, err := database.ListScriptRunLogs(config.ScriptRunLogFilter{PluginID: "plugin-timeout"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != "failed" || !strings.Contains(items[0].Error, "运行超过 1 秒") {
		t.Fatalf("unexpected timeout log: %#v", items)
	}
	select {
	case notification := <-notifications:
		if notification.LogID != items[0].ID || notification.TimeoutSeconds != 1 {
			t.Fatalf("unexpected notification: %#v", notification)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected timeout notification")
	}
	select {
	case notification := <-notifications:
		t.Fatalf("unexpected duplicate notification: %#v", notification)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestWaitScriptRunTimeoutDoesNotStopRunningScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node 不可用，跳过脚本执行测试")
	}
	pluginPath := t.TempDir()
	script := "setTimeout(() => console.log('done'), 1500)\n"
	if err := os.WriteFile(filepath.Join(pluginPath, "task.js"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(pluginPath, deps.NewManager(t.TempDir()))
	configureManagerTestProfiles(t, manager)
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	manager.SetDatabase(database)

	result := manager.RunPluginScript(pluginPath, ScriptRunAction{PluginID: "plugin-wait-timeout", Runtime: "nodejs", RuntimeProfile: "node18", Script: "task.js", Wait: true, Timeout: 1})
	if !result.Success {
		t.Fatalf("wait timeout should return success data: %#v", result)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok || data["timeout"] != true {
		t.Fatalf("expected wait timeout data, got %#v", result.Data)
	}
	var detail *config.ScriptRunLog
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		items, err := database.ListScriptRunLogs(config.ScriptRunLogFilter{PluginID: "plugin-wait-timeout"})
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 1 && items[0].Status == "success" {
			detail, err = database.GetScriptRunLog(items[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if detail == nil || !strings.Contains(detail.Output, "done") {
		t.Fatalf("script should finish after wait timeout, got %#v", detail)
	}
}

func TestPauseScriptRunKeepsRunningCancelUntilFinish(t *testing.T) {
	manager := NewManager(t.TempDir(), deps.NewManager(t.TempDir()))
	logID := int64(35)
	cancelCount := 0
	manager.runningScripts[logID] = func() { cancelCount++ }

	if !manager.PauseScriptRun(logID) {
		t.Fatal("first pause should cancel running task")
	}
	if cancelCount != 1 {
		t.Fatalf("expected one cancel call, got %d", cancelCount)
	}
	if !manager.PauseScriptRun(logID) {
		t.Fatal("second pause should remain idempotent before task finish cleanup")
	}
	if cancelCount != 2 {
		t.Fatalf("expected second cancel call before finish cleanup, got %d", cancelCount)
	}
}

func configureManagerTestProfiles(t *testing.T, manager *Manager) {
	t.Helper()
	_, err := manager.GetDepsManager().SaveRuntimeProfiles([]deps.RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: "node", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPaymentWaitAmountRawAcceptsStringJSON(t *testing.T) {
	var action struct {
		Amount json.RawMessage `json:"amount"`
	}
	if err := json.Unmarshal([]byte(`{"amount":"9.90"}`), &action); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if string(action.Amount) != `"9.90"` {
		t.Fatalf("unexpected raw amount: %s", string(action.Amount))
	}
}
