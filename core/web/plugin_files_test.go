package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginFilesImagePreviewResponse(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		writePluginWebTestPlugin(t, "shop", "商城", 1, true)
		imagePath := filepath.Join("plugins", "shop", "assets", "icon.png")
		if err := os.MkdirAll(filepath.Dir(imagePath), 0755); err != nil {
			t.Fatal(err)
		}
		imageData := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
		if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
			t.Fatal(err)
		}
		loadPluginWebTestPlugin(t, server, "shop")
		if err := server.router.RegisterPlugin(server.pluginManager.GetPlugin("shop").Plugin); err != nil {
			t.Fatal(err)
		}

		metadataRecorder := httptest.NewRecorder()
		metadataRequest := httptest.NewRequest(http.MethodGet, "/api/plugins/files/shop?path=assets%2Ficon.png", nil)
		server.handlePluginFiles(metadataRecorder, metadataRequest)
		if metadataRecorder.Code != http.StatusOK {
			t.Fatalf("metadata status = %d: %s", metadataRecorder.Code, metadataRecorder.Body.String())
		}
		var metadata struct {
			Path        string `json:"path"`
			Editable    bool   `json:"editable"`
			Text        bool   `json:"text"`
			Preview     string `json:"preview"`
			ContentType string `json:"content_type"`
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(metadataRecorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(envelope.Data, &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.Path != "assets/icon.png" || metadata.Editable || metadata.Text || metadata.Preview != "image" || metadata.ContentType != "image/png" {
			t.Fatalf("unexpected image metadata: %#v", metadata)
		}

		rawRecorder := httptest.NewRecorder()
		rawRequest := httptest.NewRequest(http.MethodGet, "/api/plugins/files/shop?path=assets%2Ficon.png&raw=1", nil)
		server.handlePluginFiles(rawRecorder, rawRequest)
		if rawRecorder.Code != http.StatusOK || rawRecorder.Header().Get("Content-Type") != "image/png" || !bytes.Equal(rawRecorder.Body.Bytes(), imageData) {
			t.Fatalf("unexpected raw image response: status=%d content-type=%q body=%v", rawRecorder.Code, rawRecorder.Header().Get("Content-Type"), rawRecorder.Body.Bytes())
		}
	})
}
