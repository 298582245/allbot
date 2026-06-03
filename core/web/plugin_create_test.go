package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/deps"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
)

const accountQLParseInputCode = `function parseInput(raw, ctx) {
  const value = String(raw || '').trim();
  if (!value) throw new Error('账号 CK 不能为空');
  return {
    envValue: value,
    uniqueKey: value,
    displayName: value.slice(0, 8)
  };
}`

const accountQLQueryCode = `async function query(account, ctx, index) {
  return String(index + 1) + '. ' + account.account_name + '｜' + (account.status || 'active');
}`

const accountQLCheckCKCode = `async function checkCk(account, ctx) {
  return { valid: true };
}`

const accountQLRouteCode = `async function withdraw(ctx, helpers) {
  await ctx.reply('提现处理中');
}`

const pythonAccountQLParseInputCode = `def parse_input(raw, ctx):
    value = str(raw or '').strip()
    if not value:
        raise RuntimeError('账号 CK 不能为空')
    return {
        'env_value': value,
        'unique_key': value,
        'display_name': value[:8],
    }`

const pythonAccountQLQueryCode = `async def query(account, ctx, index):
    return f"{index + 1}. {account.get('account_name')}｜{account.get('status') or 'active'}"`

const pythonAccountQLCheckCKCode = `async def check_ck(account, ctx):
    return {'valid': True}`

const pythonAccountQLRouteCode = `async def custom_route(ctx, helpers):
    await ctx.reply('自定义指令已执行')`

func TestNormalizeBasicCreatePluginKeepsExistingSchemaBehavior(t *testing.T) {
	fields := normalizeCreatePluginSchema([]types.PluginUserConfigField{
		{Key: " cron-key ", Default: "0 8 * * *"},
		{Key: "cron_key", Default: "duplicate"},
		{Key: "1token", Default: "abc"},
		{Key: "class", Default: "reserved"},
	})
	if len(fields) != 3 {
		t.Fatalf("expected 3 normalized fields, got %d", len(fields))
	}
	if fields[0].Key != "cron_key" || fields[0].Label != "cron_key" || fields[0].Type != "text" {
		t.Fatalf("unexpected first field: %+v", fields[0])
	}
	if fields[1].Key != "config_1token" {
		t.Fatalf("unexpected numeric field key: %s", fields[1].Key)
	}
	if fields[2].Key != "class" || configVariableName(fields[2].Key) != "config_class" {
		t.Fatalf("reserved key should keep config key and use safe variable name: %+v", fields[2])
	}
	config := normalizeCreatePluginUserConfig(fields, map[string]interface{}{"cron_key": "custom"})
	if config["cron_key"] != "custom" || config["config_1token"] != "abc" || config["class"] != "reserved" {
		t.Fatalf("unexpected user config: %#v", config)
	}
}

func TestBasicPluginTemplateUsesSafeConfigVariableNames(t *testing.T) {
	fields := []types.PluginUserConfigField{
		{Key: "class", Default: "reserved"},
		{Key: "return", Default: "reserved"},
		{Key: "normal_key", Default: "ok"},
	}
	nodeCode := nodePluginTemplate(fields)
	assertContains(t, nodeCode, "const config_class = String(ctx.config('class', \"reserved\")).trim();")
	assertContains(t, nodeCode, "const config_return = String(ctx.config('return', \"reserved\")).trim();")
	assertContains(t, nodeCode, "const normal_key = String(ctx.config('normal_key', \"ok\")).trim();")
	if strings.Contains(nodeCode, "const class =") || strings.Contains(nodeCode, "const return =") {
		t.Fatalf("node template contains reserved variable name: %s", nodeCode)
	}

	pythonCode := pythonPluginTemplate(fields)
	assertContains(t, pythonCode, "    config_class = str(ctx.config('class', \"reserved\")).strip()")
	assertContains(t, pythonCode, "    config_return = str(ctx.config('return', \"reserved\")).strip()")
	assertContains(t, pythonCode, "    normal_key = str(ctx.config('normal_key', \"ok\")).strip()")
	if strings.Contains(pythonCode, "    class =") || strings.Contains(pythonCode, "    return =") {
		t.Fatalf("python template contains reserved variable name: %s", pythonCode)
	}
}

func TestNodeAccountQLTemplateGenerateConfigAndFiles(t *testing.T) {
	withTempWorkdir(t, func() {
		req := validAccountQLCreateRequest()
		req.AccountQL.Prefix = "粉象+"
		req.AccountQL.EnableExpireCheck = boolPtr(true)
		req.AccountQL.ExpireCheckCron = "15 10 * * *"
		req.AccountQL.ExpireNotifyDays = "10,3,0"
		req.AccountQL.ExpireDeleteAfterDays = intPtr(2)
		req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "withdraw", Description: "提现", Code: accountQLRouteCode}}
		recorder := performCreatePlugin(t, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
		}

		pluginDir := filepath.Join("plugins", "ql_demo")
		config := readPluginJSON(t, pluginDir)
		if config["runtime"] != "nodejs" || config["entry"] != "main.js" {
			t.Fatalf("unexpected runtime/entry: %#v", config)
		}
		trigger := config["trigger"].(string)
		assertTriggerMatches(t, trigger, "粉象+登录", "粉象+CK检测", "粉象+过期检测", "粉象+提现")
		if regexp.MustCompile(trigger).MatchString("粉象象登录") {
			t.Fatalf("trigger should escape regex special chars: %s", trigger)
		}
		dependencies := config["dependencies"].(map[string]interface{})
		if len(dependencies) != 0 {
			t.Fatalf("template should not add dependencies: %#v", dependencies)
		}

		schema := config["user_config_schema"].([]interface{})
		assertSchemaKeys(t, schema, "task_script", "script_runtime", "auth_price_per_month", "cron", "run_wait_timeout", "ck_check_cron", "expire_check_cron", "expire_notify_days", "expire_delete_after_days")
		userConfig := config["user_config"].(map[string]interface{})
		if userConfig["task_script"] != "scripts/demo_task.js" || userConfig["script_runtime"] != "nodejs" || int(userConfig["run_wait_timeout"].(float64)) != 7200 || userConfig["expire_notify_days"] != "10,3,0" || int(userConfig["expire_delete_after_days"].(float64)) != 2 {
			t.Fatalf("unexpected user_config: %#v", userConfig)
		}

		mainJS := readTextFile(t, filepath.Join(pluginDir, "main.js"))
		assertContains(t, mainJS, "createAccountQLPlugin")
		assertContains(t, mainJS, "builtinPointsAuth")
		assertContains(t, mainJS, "runtime: \"nodejs\"")
		assertContains(t, mainJS, "runtimeConfig: 'script_runtime'")
		assertContains(t, mainJS, "scriptConfig: 'task_script'")
		assertContains(t, mainJS, "timeoutConfig: 'run_wait_timeout'")
		assertContains(t, mainJS, "function parseInput")
		assertContains(t, mainJS, "async function query")
		assertContains(t, mainJS, "async function checkCk")
		assertContains(t, mainJS, "async function withdraw")
		assertContains(t, mainJS, "\"提现\": withdraw")
		assertContains(t, mainJS, "expireCheck")
		assertContains(t, mainJS, "ckCheck")

		taskScript := readTextFile(t, filepath.Join(pluginDir, "scripts", "demo_task.js"))
		assertContains(t, taskScript, "const envName = \"DEMO_CK\";")
		assertContains(t, taskScript, "账号数量")
	})
}

func TestNodeAccountQLTemplateCanUsePythonTaskScript(t *testing.T) {
	withTempWorkdir(t, func() {
		req := validAccountQLCreateRequest()
		req.AccountQL.ScriptRuntime = "python"
		req.AccountQL.TaskScript = "scripts/demo_task.py"
		recorder := performCreatePlugin(t, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
		}

		pluginDir := filepath.Join("plugins", "ql_demo")
		config := readPluginJSON(t, pluginDir)
		if config["runtime"] != "nodejs" || config["entry"] != "main.js" {
			t.Fatalf("unexpected runtime/entry: %#v", config)
		}
		userConfig := config["user_config"].(map[string]interface{})
		if userConfig["task_script"] != "scripts/demo_task.py" || userConfig["script_runtime"] != "python" {
			t.Fatalf("unexpected user_config: %#v", userConfig)
		}

		mainJS := readTextFile(t, filepath.Join(pluginDir, "main.js"))
		assertContains(t, mainJS, "runtime: \"python\"")
		assertContains(t, mainJS, "runtimeConfig: 'script_runtime'")
		assertContains(t, mainJS, "scriptConfig: 'task_script'")

		taskScript := readTextFile(t, filepath.Join(pluginDir, "scripts", "demo_task.py"))
		assertContains(t, taskScript, "import os")
		assertContains(t, taskScript, "env_name = \"DEMO_CK\"")
		assertContains(t, taskScript, "账号数量")
	})
}

func TestPythonAccountQLTemplateGenerateConfigAndFiles(t *testing.T) {
	withTempWorkdir(t, func() {
		req := validPythonAccountQLCreateRequest()
		req.AccountQL.EnableExpireCheck = boolPtr(true)
		req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "custom_route", Description: "提现", Code: pythonAccountQLRouteCode}}
		recorder := performCreatePlugin(t, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
		}

		pluginDir := filepath.Join("plugins", "ql_demo")
		config := readPluginJSON(t, pluginDir)
		if config["runtime"] != "python" || config["entry"] != "main.py" {
			t.Fatalf("unexpected runtime/entry: %#v", config)
		}
		assertTriggerMatches(t, config["trigger"].(string), "演示登录", "演示CK检测", "演示过期检测", "演示提现")
		assertSchemaKeys(t, config["user_config_schema"].([]interface{}), "task_script", "script_runtime", "auth_price_per_month", "cron", "run_wait_timeout", "ck_check_cron", "expire_check_cron", "expire_notify_days", "expire_delete_after_days")
		userConfig := config["user_config"].(map[string]interface{})
		if userConfig["task_script"] != "scripts/demo_task.py" || userConfig["script_runtime"] != "python" || int(userConfig["run_wait_timeout"].(float64)) != 7200 {
			t.Fatalf("unexpected user_config: %#v", userConfig)
		}

		mainPY := readTextFile(t, filepath.Join(pluginDir, "main.py"))
		assertContains(t, mainPY, "create_account_ql_plugin")
		assertContains(t, mainPY, "builtin_points_auth")
		assertContains(t, mainPY, "def parse_input")
		assertContains(t, mainPY, "async def query")
		assertContains(t, mainPY, "async def check_ck")
		assertContains(t, mainPY, "async def custom_route")
		assertContains(t, mainPY, "\"提现\": custom_route")
		assertContains(t, mainPY, "\"runtime\": \"python\"")
		assertContains(t, mainPY, "\"runtime_config\": \"script_runtime\"")
		assertContains(t, mainPY, "\"script_config\": \"task_script\"")
		assertContains(t, mainPY, "\"timeout_config\": \"run_wait_timeout\"")
		assertContains(t, mainPY, "\"expire_check\"")
		assertContains(t, mainPY, "\"ck_check\"")

		taskScript := readTextFile(t, filepath.Join(pluginDir, "scripts", "demo_task.py"))
		assertContains(t, taskScript, "env_name = \"DEMO_CK\"")
		assertContains(t, taskScript, "账号数量")
	})
}

func TestPluginTemplatesAPI(t *testing.T) {
	server := testServer(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/plugins/templates", nil)
	server.handlePluginTemplates(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 templates, got %d: %#v", len(result), result)
	}
	ids := map[string]bool{}
	for _, item := range result {
		id := item["id"].(string)
		ids[id] = true
		if item["version"] == "" || item["runtime"] == "" {
			t.Fatalf("template missing version/runtime: %#v", item)
		}
		if id == "nodejs_account_ql" || id == "python_account_ql" {
			defaults := item["defaults"].(map[string]interface{})
			if defaults["script_runtime"] == "" {
				t.Fatalf("template missing script_runtime default: %#v", item)
			}
		}
	}
	for _, id := range []string{"basic", "nodejs_account_ql", "python_account_ql"} {
		if !ids[id] {
			t.Fatalf("missing template %s: %#v", id, result)
		}
	}
}

func TestCreatePluginPreviewDoesNotCreateFiles(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins/preview", validAccountQLCreateRequest(), server.handlePluginCreatePreview)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
		}
		if _, err := os.Stat(filepath.Join("plugins", "ql_demo")); !os.IsNotExist(err) {
			t.Fatalf("preview should not create plugin directory, stat err=%v", err)
		}
	})
}

func TestCreatePluginPreviewReturnsGeneratedFiles(t *testing.T) {
	server := testServer(t)
	recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins/preview", validPythonAccountQLCreateRequest(), server.handlePluginCreatePreview)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["plugin_id"] != "ql_demo" || result["template"] != "python_account_ql" || result["entry"] != "main.py" {
		t.Fatalf("unexpected preview summary: %#v", result)
	}
	files := result["files"].([]interface{})
	paths := map[string]string{}
	for _, item := range files {
		file := item.(map[string]interface{})
		paths[file["path"].(string)] = file["content"].(string)
	}
	assertContains(t, paths["plugin.json"], "\"template\": \"python_account_ql\"")
	assertContains(t, paths["main.py"], "create_account_ql_plugin")
	assertContains(t, paths["scripts/demo_task.py"], "账号数量")
}

func TestCreatePluginValidateAcceptsAccountQLTemplates(t *testing.T) {
	server := testServer(t)
	recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins/validate", validAccountQLCreateRequest(), server.handlePluginCreateValidate)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true {
		t.Fatalf("expected ok validate response: %#v", result)
	}
}

func TestCreatePluginValidateAcceptsNodePluginWithPythonTaskScript(t *testing.T) {
	server := testServer(t)
	req := validAccountQLCreateRequest()
	req.AccountQL.ScriptRuntime = "python"
	req.AccountQL.TaskScript = "scripts/demo_task.py"
	recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins/validate", req, server.handlePluginCreateValidate)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true {
		t.Fatalf("expected ok validate response: %#v", result)
	}
	normalized := result["normalized"].(map[string]interface{})
	if normalized["runtime"] != "nodejs" || normalized["entry"] != "main.js" || normalized["script_runtime"] != "python" || normalized["task_script"] != "scripts/demo_task.py" {
		t.Fatalf("unexpected normalized response: %#v", normalized)
	}
}

func TestCreatePluginValidateRejectsInvalidCron(t *testing.T) {
	server := testServer(t)
	req := validAccountQLCreateRequest()
	req.AccountQL.Cron = "bad cron"
	recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins/validate", req, server.handlePluginCreateValidate)
	var result map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["ok"] != false {
		t.Fatalf("expected invalid cron response: %#v", result)
	}
	errors := result["errors"].([]interface{})
	if len(errors) == 0 || errors[0].(map[string]interface{})["field"] != "account_ql.cron" {
		t.Fatalf("unexpected errors: %#v", errors)
	}
}

func TestCreatePluginValidateRejectsInvalidTaskScript(t *testing.T) {
	server := testServer(t)
	req := validAccountQLCreateRequest()
	req.AccountQL.TaskScript = "../task.js"
	recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins/validate", req, server.handlePluginCreateValidate)
	var result map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["ok"] != false {
		t.Fatalf("expected invalid task script response: %#v", result)
	}
	errors := result["errors"].([]interface{})
	if len(errors) == 0 || errors[0].(map[string]interface{})["field"] != "account_ql.task_script" {
		t.Fatalf("unexpected errors: %#v", errors)
	}
}

func TestCreatePluginReturnsDiagnosticsAndMetadata(t *testing.T) {
	withTempWorkdir(t, func() {
		recorder := performCreatePlugin(t, validAccountQLCreateRequest())
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var result map[string]interface{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["id"] != "ql_demo" || result["plugin_id"] != "ql_demo" || result["template"] != "nodejs_account_ql" {
			t.Fatalf("unexpected create response: %#v", result)
		}
		if diagnostics := result["diagnostics"].([]interface{}); len(diagnostics) < 5 {
			t.Fatalf("expected diagnostics, got %#v", diagnostics)
		}
		metadata := result["metadata"].(map[string]interface{})
		if metadata["env_name"] != "DEMO_CK" || metadata["script_runtime"] != "nodejs" || metadata["structure"] != "account_ql" {
			t.Fatalf("unexpected metadata: %#v", metadata)
		}
		config := readPluginJSON(t, filepath.Join("plugins", "ql_demo"))
		if config["template"] != "nodejs_account_ql" || config["template_version"] == "" || config["template_metadata"] == nil {
			t.Fatalf("plugin.json missing template metadata: %#v", config)
		}
	})
}

func TestAccountQLTemplateDisabledChecksOmitOptionalConfig(t *testing.T) {
	withTempWorkdir(t, func() {
		req := validAccountQLCreateRequest()
		req.AccountQL.EnableCKCheck = boolPtr(false)
		req.AccountQL.EnableExpireCheck = boolPtr(false)
		req.AccountQL.CheckCKCode = ""
		recorder := performCreatePlugin(t, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
		}
		pluginDir := filepath.Join("plugins", "ql_demo")
		config := readPluginJSON(t, pluginDir)
		trigger := config["trigger"].(string)
		assertTriggerMatches(t, trigger, "演示登录")
		assertTriggerNotMatches(t, trigger, "演示CK检测", "演示过期检测")
		assertSchemaKeys(t, config["user_config_schema"].([]interface{}), "task_script", "script_runtime", "auth_price_per_month", "cron", "run_wait_timeout")
		mainJS := readTextFile(t, filepath.Join(pluginDir, "main.js"))
		assertNotContains(t, mainJS, "checkCk")
		assertNotContains(t, mainJS, "ckCheck")
		assertNotContains(t, mainJS, "expireCheck")
	})
}

func TestNodeAccountQLTemplateDefaults(t *testing.T) {
	req := validAccountQLCreateRequest()
	req.AccountQL.Cron = ""
	req.AccountQL.CKCheckCron = ""
	req.AccountQL.ExpireCheckCron = ""
	req.AccountQL.ExpireNotifyDays = ""
	req.AccountQL.RunWaitTimeout = 0
	req.AccountQL.AuthPricePerMonth = -10
	options, err := normalizeNodeAccountQLTemplate("ql_demo", &req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if options.Cron != "0 8 * * *" || options.CKCheckCron != "25 9 * * *" || options.ExpireCheckCron != "15 9 * * *" || options.ExpireNotifyDays != "7,3,1,0" || options.RunWaitTimeout != 7200 || options.AuthPricePerMonth != 0 || options.ExpireDeleteAfterDays != -1 {
		t.Fatalf("unexpected defaults: %+v", options)
	}
}

func TestNodeAccountQLTemplateValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*createPluginRequest)
	}{
		{name: "invalid env", mutate: func(req *createPluginRequest) { req.AccountQL.EnvName = "1_BAD" }},
		{name: "invalid table", mutate: func(req *createPluginRequest) { req.AccountQL.TableName = "bad-name" }},
		{name: "absolute script", mutate: func(req *createPluginRequest) { req.AccountQL.TaskScript = "/tmp/task.js" }},
		{name: "windows absolute script", mutate: func(req *createPluginRequest) { req.AccountQL.TaskScript = "C:/tmp/task.js" }},
		{name: "parent script", mutate: func(req *createPluginRequest) { req.AccountQL.TaskScript = "../task.js" }},
		{name: "entry script", mutate: func(req *createPluginRequest) { req.AccountQL.TaskScript = "main.js" }},
		{name: "script runtime mismatch", mutate: func(req *createPluginRequest) {
			req.AccountQL.ScriptRuntime = "nodejs"
			req.AccountQL.TaskScript = "scripts/task.py"
		}},
		{name: "invalid script runtime", mutate: func(req *createPluginRequest) { req.AccountQL.ScriptRuntime = "ruby" }},
		{name: "missing parseInput", mutate: func(req *createPluginRequest) { req.AccountQL.ParseInputCode = "function other() {}" }},
		{name: "parseInput only in comment", mutate: func(req *createPluginRequest) {
			req.AccountQL.ParseInputCode = "// function parseInput(raw, ctx) {}\nfunction other() {}"
		}},
		{name: "parseInput as longer name", mutate: func(req *createPluginRequest) { req.AccountQL.ParseInputCode = "function parseInputOld(raw, ctx) {}" }},
		{name: "missing query", mutate: func(req *createPluginRequest) { req.AccountQL.QueryCode = "async function other() {}" }},
		{name: "missing checkCk", mutate: func(req *createPluginRequest) { req.AccountQL.CheckCKCode = "async function other() {}" }},
		{name: "python runtime", mutate: func(req *createPluginRequest) { req.Runtime = "python" }},
		{name: "builtin route command", mutate: func(req *createPluginRequest) {
			req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "登录", FunctionName: "customRoute", Code: "async function customRoute(ctx, plugin) {}"}}
		}},
		{name: "duplicate route command", mutate: func(req *createPluginRequest) {
			req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "routeA", Code: "async function routeA(ctx, plugin) {}"}, {Command: "提现", FunctionName: "routeB", Code: "async function routeB(ctx, plugin) {}"}}
		}},
		{name: "invalid route function", mutate: func(req *createPluginRequest) {
			req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "1bad", Code: "async function 1bad(ctx, plugin) {}"}}
		}},
		{name: "reserved route function", mutate: func(req *createPluginRequest) {
			req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "return", Code: "async function return(ctx, plugin) {}"}}
		}},
		{name: "missing route function", mutate: func(req *createPluginRequest) {
			req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "withdraw", Code: "async function other(ctx, plugin) {}"}}
		}},
		{name: "route function only in comment", mutate: func(req *createPluginRequest) {
			req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "withdraw", Code: "// async function withdraw(ctx, plugin) {}\nasync function other(ctx, plugin) {}"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validAccountQLCreateRequest()
			tc.mutate(&req)
			if _, err := normalizeNodeAccountQLTemplate("ql_demo", &req); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestPythonAccountQLTemplateValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*createPluginRequest)
	}{
		{name: "node runtime", mutate: func(req *createPluginRequest) { req.Runtime = "nodejs" }},
		{name: "entry script", mutate: func(req *createPluginRequest) { req.AccountQL.TaskScript = "main.py" }},
		{name: "script runtime mismatch", mutate: func(req *createPluginRequest) {
			req.AccountQL.ScriptRuntime = "python"
			req.AccountQL.TaskScript = "scripts/task.js"
		}},
		{name: "missing parse_input", mutate: func(req *createPluginRequest) { req.AccountQL.ParseInputCode = "def other():\n    pass" }},
		{name: "parse_input only in comment", mutate: func(req *createPluginRequest) {
			req.AccountQL.ParseInputCode = "# def parse_input(raw, ctx):\n#     pass\ndef other():\n    pass"
		}},
		{name: "missing query", mutate: func(req *createPluginRequest) { req.AccountQL.QueryCode = "async def other():\n    pass" }},
		{name: "missing check_ck", mutate: func(req *createPluginRequest) { req.AccountQL.CheckCKCode = "async def other():\n    pass" }},
		{name: "invalid route function", mutate: func(req *createPluginRequest) {
			req.AccountQL.Routes = []createAccountQLRouteRequest{{Command: "提现", FunctionName: "bad-name", Code: "async def bad_name(ctx, plugin):\n    pass"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validPythonAccountQLCreateRequest()
			tc.mutate(&req)
			if _, err := normalizeAccountQLTemplate("ql_demo", "python_account_ql", &req); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func validAccountQLCreateRequest() createPluginRequest {
	return createPluginRequest{
		ID:        "ql_demo",
		Name:      "青龙演示",
		Version:   "1.0.0",
		Runtime:   "nodejs",
		Template:  "nodejs_account_ql",
		Priority:  10,
		Platforms: []string{"qq", "telegram"},
		Enabled:   true,
		AccountQL: &createAccountQLRequest{
			Prefix:            "演示",
			TableName:         "demo_accounts",
			EnvName:           "DEMO_CK",
			TaskScript:        "scripts/demo_task.js",
			ScriptRuntime:     "nodejs",
			AuthPricePerMonth: 3,
			Cron:              "5 8 * * *",
			CKCheckCron:       "25 9 * * *",
			RunWaitTimeout:    7200,
			ParseInputCode:    accountQLParseInputCode,
			QueryCode:         accountQLQueryCode,
			CheckCKCode:       accountQLCheckCKCode,
		},
	}
}

func validPythonAccountQLCreateRequest() createPluginRequest {
	req := validAccountQLCreateRequest()
	req.Runtime = "python"
	req.Template = "python_account_ql"
	req.AccountQL.TaskScript = "scripts/demo_task.py"
	req.AccountQL.ScriptRuntime = "python"
	req.AccountQL.ParseInputCode = pythonAccountQLParseInputCode
	req.AccountQL.QueryCode = pythonAccountQLQueryCode
	req.AccountQL.CheckCKCode = pythonAccountQLCheckCKCode
	return req
}

func performCreatePlugin(t *testing.T, req createPluginRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	server := testServer(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/plugins", strings.NewReader(string(body)))
	server.handleCreatePlugin(recorder, request)
	return recorder
}

func performPluginCreateRequest(t *testing.T, server *Server, method string, path string, req createPluginRequest, handler func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	handler(recorder, request)
	return recorder
}

func TestBackupAndDeletePluginDisablesPluginScheduledTasks(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		pluginDir := filepath.Join("plugins", "demo")
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"id":"demo","name":"Demo"}`), 0644); err != nil {
			t.Fatal(err)
		}
		nextRunAt := time.Now().Add(time.Hour)
		database := server.adapterManager.GetDatabase()
		items := []*config.ScheduledTask{
			{PluginID: "demo", TaskKey: "plugin-task", Name: "插件任务", Enabled: true, Cron: "0 8 * * *", Platform: "qq", UserID: "1001", Content: "插件消息", Source: "plugin", NextRunAt: &nextRunAt},
			{PluginID: "demo", TaskKey: "admin-task", Name: "管理员任务", Enabled: true, Cron: "0 9 * * *", Platform: "qq", UserID: "1002", Content: "管理员消息", Source: "user", NextRunAt: &nextRunAt},
		}
		for _, item := range items {
			if err := database.SaveScheduledTask(item); err != nil {
				t.Fatal(err)
			}
		}

		if _, err := server.backupAndDeletePlugin("demo"); err != nil {
			t.Fatal(err)
		}

		stored, err := database.ListScheduledTasks()
		if err != nil {
			t.Fatal(err)
		}
		byKey := map[string]*config.ScheduledTask{}
		for _, item := range stored {
			byKey[item.TaskKey] = item
		}
		if task := byKey["plugin-task"]; task == nil || task.Enabled || task.NextRunAt != nil {
			t.Fatalf("plugin task should be disabled after plugin deletion: %#v", task)
		}
		if task := byKey["admin-task"]; task == nil || !task.Enabled || task.NextRunAt == nil {
			t.Fatalf("admin task should stay enabled after plugin deletion: %#v", task)
		}
		if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
			t.Fatalf("plugin directory should be deleted, stat err=%v", err)
		}
	})
}

func testServer(t *testing.T) *Server {
	t.Helper()
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	depsManager := deps.NewManager("runtime")
	pluginManager := plugincore.NewManager("plugins", depsManager)
	pluginManager.SetDatabase(database)
	r := router.NewRouter(session.NewManager())
	adapterManager := config.NewAdapterManager(database)
	return NewServer("0", pluginManager, r, adapterManager, nil)
}

func withTempWorkdir(t *testing.T, fn func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

func readPluginJSON(t *testing.T, pluginDir string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertContains(t *testing.T, value, expected string) {
	t.Helper()
	if !strings.Contains(value, expected) {
		t.Fatalf("expected %q to contain %q", value, expected)
	}
}

func assertNotContains(t *testing.T, value, expected string) {
	t.Helper()
	if strings.Contains(value, expected) {
		t.Fatalf("expected %q not to contain %q", value, expected)
	}
}

func assertTriggerMatches(t *testing.T, trigger string, values ...string) {
	t.Helper()
	compiled, err := regexp.Compile(trigger)
	if err != nil {
		t.Fatalf("trigger should compile: %v", err)
	}
	for _, value := range values {
		if !compiled.MatchString(value) {
			t.Fatalf("trigger %s should match %s", trigger, value)
		}
	}
}

func assertTriggerNotMatches(t *testing.T, trigger string, values ...string) {
	t.Helper()
	compiled := regexp.MustCompile(trigger)
	for _, value := range values {
		if compiled.MatchString(value) {
			t.Fatalf("trigger %s should not match %s", trigger, value)
		}
	}
}

func assertSchemaKeys(t *testing.T, schema []interface{}, expected ...string) {
	t.Helper()
	if len(schema) != len(expected) {
		t.Fatalf("expected %d config fields, got %d: %#v", len(expected), len(schema), schema)
	}
	actual := map[string]bool{}
	for _, item := range schema {
		field, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected schema field: %#v", item)
		}
		actual[field["key"].(string)] = true
	}
	for _, key := range expected {
		if !actual[key] {
			t.Fatalf("expected schema to contain key %s: %#v", key, schema)
		}
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}
