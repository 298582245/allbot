package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/imagehost"
)

func TestHandleImagesUploadAndOpen(t *testing.T) {
	server, cleanup := newImageHostTestServer(t)
	defer cleanup()

	recorder := httptest.NewRecorder()
	request := newImageUploadRequest(t, "/api/images", "demo.png", testWebPNG(t))
	server.handleImages(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var asset imagehost.ImageAssetResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &asset); err != nil {
		t.Fatal(err)
	}
	if asset.PublicID == "" || asset.URL == "" || asset.ContentType != "image/png" {
		t.Fatalf("asset unexpected: %+v", asset)
	}

	openRecorder := httptest.NewRecorder()
	openRequest := httptest.NewRequest(http.MethodGet, "/api/open/images/"+asset.PublicID+".png", nil)
	server.handleOpenImage(openRecorder, openRequest)
	if openRecorder.Code != http.StatusOK || openRecorder.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected public png, got %d %s", openRecorder.Code, openRecorder.Header().Get("Content-Type"))
	}
}

func TestHandleImagesUploadRejectsBadRequests(t *testing.T) {
	server, cleanup := newImageHostTestServer(t)
	defer cleanup()
	cases := []struct {
		name    string
		request *http.Request
	}{
		{"non multipart", httptest.NewRequest(http.MethodPost, "/api/images", strings.NewReader("bad"))},
		{"missing file", newMultipartRequestWithoutFile(t)},
		{"not image", newImageUploadRequest(t, "/api/images", "bad.txt", []byte("not image"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.handleImages(recorder, tc.request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestHandleImagesListSettingsAndDelete(t *testing.T) {
	server, cleanup := newImageHostTestServer(t)
	defer cleanup()
	uploadRecorder := httptest.NewRecorder()
	server.handleImages(uploadRecorder, newImageUploadRequest(t, "/api/images", "demo.png", testWebPNG(t)))
	if uploadRecorder.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	var asset imagehost.ImageAssetResponse
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &asset); err != nil {
		t.Fatal(err)
	}

	listRecorder := httptest.NewRecorder()
	server.handleImages(listRecorder, httptest.NewRequest(http.MethodGet, "/api/images?keyword=demo&content_type=image/png", nil))
	if listRecorder.Code != http.StatusOK || !strings.Contains(listRecorder.Body.String(), asset.PublicID) {
		t.Fatalf("list failed: %d %s", listRecorder.Code, listRecorder.Body.String())
	}

	settingsRecorder := httptest.NewRecorder()
	server.handleImageDetail(settingsRecorder, httptest.NewRequest(http.MethodGet, "/api/images/settings", nil))
	if settingsRecorder.Code != http.StatusOK || !strings.Contains(settingsRecorder.Body.String(), "storage_dir") {
		t.Fatalf("settings failed: %d %s", settingsRecorder.Code, settingsRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	server.handleImageDetail(deleteRecorder, httptest.NewRequest(http.MethodDelete, "/api/images/"+asset.PublicID, nil))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	openRecorder := httptest.NewRecorder()
	server.handleOpenImage(openRecorder, httptest.NewRequest(http.MethodGet, "/api/open/images/"+asset.PublicID+".png", nil))
	if openRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected public not found after delete, got %d", openRecorder.Code)
	}
}

func TestImageAuthMiddleware(t *testing.T) {
	server, cleanup := newImageHostTestServer(t)
	defer cleanup()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/images", server.handleImages)
	mux.HandleFunc("/api/open/images/", server.handleOpenImage)
	handler := server.authMiddleware(mux)

	privateRecorder := httptest.NewRecorder()
	handler.ServeHTTP(privateRecorder, httptest.NewRequest(http.MethodGet, "/api/images", nil))
	if privateRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected private unauthorized, got %d", privateRecorder.Code)
	}
	publicRecorder := httptest.NewRecorder()
	handler.ServeHTTP(publicRecorder, httptest.NewRequest(http.MethodGet, "/api/open/images/missing.png", nil))
	if publicRecorder.Code == http.StatusUnauthorized {
		t.Fatal("public image path should bypass auth")
	}
}

func TestHandleImageSettingsKeepOldReturnsMigration(t *testing.T) {
	server, cleanup := newImageHostTestServer(t)
	defer cleanup()
	oldSettings, err := server.imageHostService.Settings()
	if err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(filepath.Dir(oldSettings.StorageDir), "new")
	payload := map[string]interface{}{"storage_dir": newDir, "public_base_url": "", "max_size": oldSettings.MaxSize, "allowed_types": oldSettings.AllowedTypes, "storage_dir_action": "keep_old"}

	recorder := httptest.NewRecorder()
	server.handleImageDetail(recorder, newJSONRequest(t, http.MethodPut, "/api/images/settings", payload))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		StorageDir string `json:"storage_dir"`
		Migration  struct {
			Changed bool   `json:"changed"`
			Action  string `json:"action"`
		} `json:"migration"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.StorageDir != newDir || !response.Migration.Changed || response.Migration.Action != "keep_old" {
		t.Fatalf("response unexpected: %+v", response)
	}
}

func TestHandleImageSettingsMigrateKeepsPublicImageAvailable(t *testing.T) {
	server, cleanup := newImageHostTestServer(t)
	defer cleanup()
	uploadRecorder := httptest.NewRecorder()
	server.handleImages(uploadRecorder, newImageUploadRequest(t, "/api/images", "demo.png", testWebPNG(t)))
	if uploadRecorder.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	var asset imagehost.ImageAssetResponse
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &asset); err != nil {
		t.Fatal(err)
	}
	oldSettings, err := server.imageHostService.Settings()
	if err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(filepath.Dir(oldSettings.StorageDir), "new")
	payload := map[string]interface{}{"storage_dir": newDir, "public_base_url": "", "max_size": oldSettings.MaxSize, "allowed_types": oldSettings.AllowedTypes, "storage_dir_action": "migrate_delete_old"}

	settingsRecorder := httptest.NewRecorder()
	server.handleImageDetail(settingsRecorder, newJSONRequest(t, http.MethodPut, "/api/images/settings", payload))
	if settingsRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", settingsRecorder.Code, settingsRecorder.Body.String())
	}
	openRecorder := httptest.NewRecorder()
	server.handleOpenImage(openRecorder, httptest.NewRequest(http.MethodGet, "/api/open/images/"+asset.PublicID+".png", nil))
	if openRecorder.Code != http.StatusOK {
		t.Fatalf("expected public image available, got %d: %s", openRecorder.Code, openRecorder.Body.String())
	}
}

func TestHandleImageSettingsRejectsInvalidAction(t *testing.T) {
	server, cleanup := newImageHostTestServer(t)
	defer cleanup()
	oldSettings, err := server.imageHostService.Settings()
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]interface{}{"storage_dir": filepath.Join(filepath.Dir(oldSettings.StorageDir), "new"), "public_base_url": "", "max_size": oldSettings.MaxSize, "allowed_types": oldSettings.AllowedTypes, "storage_dir_action": "bad"}

	recorder := httptest.NewRecorder()
	server.handleImageDetail(recorder, newJSONRequest(t, http.MethodPut, "/api/images/settings", payload))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func newImageHostTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	dir := t.TempDir()
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = dir
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	adapterManager := config.NewAdapterManager(db)
	server := NewServer("0", nil, nil, adapterManager, nil)
	server.SetImageHostService(imagehost.NewService(db))
	return server, func() { _ = db.Close() }
}

func newJSONRequest(t *testing.T, method, path string, payload interface{}) *http.Request {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func newImageUploadRequest(t *testing.T, path, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("name", filepath.Base(filename)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Host = "example.test"
	return request
}

func newMultipartRequestWithoutFile(t *testing.T) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("name", "demo"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/images", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func testWebPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
