package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/allbot/allbot/core/deps"
)

func TestHandleDependenciesReadsProfileDependencies(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		depsManager := server.pluginManager.GetDepsManager()
		profiles, err := depsManager.SaveRuntimeProfiles([]deps.RuntimeProfile{
			{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
			{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: "node", Enabled: true},
			{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, profile := range profiles {
			if profile.ID != "node18" {
				continue
			}
			depsFile := filepath.Join("runtime", "envs", profile.ID, "package.json")
			if err := os.MkdirAll(filepath.Dir(depsFile), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(depsFile, []byte(`{"dependencies":{"axios":"1.7.0"}}`), 0644); err != nil {
				t.Fatal(err)
			}
		}

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/dependencies?runtime=nodejs&profile_id=node18", nil)
		server.handleDependencies(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Runtime      string            `json:"runtime"`
			ProfileID    string            `json:"profile_id"`
			Dependencies map[string]string `json:"dependencies"`
		}
		decodeUnifiedResponseData(t, recorder, &response)
		if response.Runtime != "nodejs" || response.ProfileID != "node18" || response.Dependencies["axios"] != "1.7.0" {
			t.Fatalf("unexpected response: %#v", response)
		}
	})
}

func TestHandleDependenciesRejectsUnknownRuntime(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/dependencies?runtime=ruby", nil)
		server.handleDependencies(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
	})
}
