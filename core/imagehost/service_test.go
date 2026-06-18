package imagehost

import (
	"bytes"
	"database/sql"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/config"
)

func TestUploadImagesAndPublicURL(t *testing.T) {
	service, db, dir := newImageHostTestService(t)
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = dir
	settings.PublicBaseURL = "https://img.example.com/"
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		data        []byte
		contentType string
		ext         string
	}{
		{"png", testPNG(t), "image/png", "png"},
		{"jpg", testJPEG(t), "image/jpeg", "jpg"},
		{"gif", testGIF(t), "image/gif", "gif"},
		{"webp", testWebP(), "image/webp", "webp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asset, err := service.Upload(UploadInput{Reader: bytes.NewReader(tc.data), OriginalName: `..\\evil/` + tc.name + ".txt", RequestHost: "localhost:8080"})
			if err != nil {
				t.Fatalf("Upload returned error: %v", err)
			}
			if asset.ContentType != tc.contentType || asset.Ext != tc.ext || !strings.HasPrefix(asset.URL, "https://img.example.com/api/open/images/") {
				t.Fatalf("asset unexpected: %+v", asset)
			}
			if strings.Contains(asset.OriginalName, "\\") || strings.Contains(asset.OriginalName, "/") {
				t.Fatalf("original name was not sanitized: %q", asset.OriginalName)
			}
			if _, err := os.Stat(filepath.Join(dir, asset.StorageKey)); err != nil {
				t.Fatalf("stored file missing: %v", err)
			}
		})
	}
}

func TestUploadRejectsNonImageAndOversize(t *testing.T) {
	service, db, dir := newImageHostTestService(t)
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = dir
	settings.MaxSize = 8
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Upload(UploadInput{Reader: strings.NewReader("not-image"), OriginalName: "a.txt"}); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for non image, got %v", err)
	}
	if _, err := service.Upload(UploadInput{Reader: bytes.NewReader(testPNG(t)), OriginalName: "a.png"}); err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for oversize, got %v", err)
	}
}

func TestResolvePublicAndDelete(t *testing.T) {
	service, db, dir := newImageHostTestService(t)
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = dir
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	asset, err := service.Upload(UploadInput{Reader: bytes.NewReader(testPNG(t)), OriginalName: "demo.png"})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	resolved, err := service.ResolvePublic(asset.PublicID + "." + asset.Ext)
	if err != nil {
		t.Fatalf("ResolvePublic returned error: %v", err)
	}
	if resolved.ContentType != "image/png" || resolved.Path == "" {
		t.Fatalf("resolved unexpected: %+v", resolved)
	}
	if err := service.Delete(asset.PublicID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, asset.StorageKey)); !os.IsNotExist(err) {
		t.Fatalf("expected stored file removed, got %v", err)
	}
	if _, err := db.GetImageAssetByPublicID(asset.PublicID); err != sql.ErrNoRows {
		t.Fatalf("expected metadata removed, got %v", err)
	}
	if _, err := service.ResolvePublic(asset.PublicID + "." + asset.Ext); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestSaveSettingsWithOptionsKeepOld(t *testing.T) {
	service, db, baseDir := newImageHostTestService(t)
	oldDir := filepath.Join(baseDir, "old")
	newDir := filepath.Join(baseDir, "new")
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = oldDir
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	asset, err := service.Upload(UploadInput{Reader: bytes.NewReader(testPNG(t)), OriginalName: "demo.png"})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	settings.StorageDir = newDir
	result, err := service.SaveSettingsWithOptions(settings, SaveSettingsOptions{StorageDirAction: StorageDirActionKeepOld})
	if err != nil {
		t.Fatalf("SaveSettingsWithOptions returned error: %v", err)
	}
	if !result.Migration.Changed || result.Migration.Action != StorageDirActionKeepOld || result.Migration.Warning == "" {
		t.Fatalf("migration unexpected: %+v", result.Migration)
	}
	if _, err := os.Stat(filepath.Join(oldDir, asset.StorageKey)); err != nil {
		t.Fatalf("old file missing: %v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("new dir missing: %v", err)
	}
	loaded, err := db.GetImageHostSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StorageDir != newDir {
		t.Fatalf("expected new storage dir, got %+v", loaded)
	}
	if _, err := service.ResolvePublic(asset.PublicID + "." + asset.Ext); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected old public image unavailable, got %v", err)
	}
}

func TestSaveSettingsWithOptionsMigrateDeleteOld(t *testing.T) {
	service, db, baseDir := newImageHostTestService(t)
	oldDir := filepath.Join(baseDir, "old")
	newDir := filepath.Join(baseDir, "new")
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = oldDir
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	asset, err := service.Upload(UploadInput{Reader: bytes.NewReader(testPNG(t)), OriginalName: "demo.png"})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	settings.StorageDir = newDir
	result, err := service.SaveSettingsWithOptions(settings, SaveSettingsOptions{StorageDirAction: StorageDirActionMigrateDeleteOld})
	if err != nil {
		t.Fatalf("SaveSettingsWithOptions returned error: %v", err)
	}
	if !result.Migration.Changed || result.Migration.MigratedFiles != 1 || !result.Migration.DeletedOldDir {
		t.Fatalf("migration unexpected: %+v", result.Migration)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("expected old dir removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, asset.StorageKey)); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
	if _, err := service.ResolvePublic(asset.PublicID + "." + asset.Ext); err != nil {
		t.Fatalf("expected public image available after migrate, got %v", err)
	}
}

func TestSaveSettingsWithOptionsSameDirIgnoresAction(t *testing.T) {
	service, db, dir := newImageHostTestService(t)
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = dir
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings.PublicBaseURL = "https://img.example.com/"
	result, err := service.SaveSettingsWithOptions(settings, SaveSettingsOptions{StorageDirAction: "bad"})
	if err != nil {
		t.Fatalf("SaveSettingsWithOptions returned error: %v", err)
	}
	if result.Migration.Changed || result.Migration.Action != "" {
		t.Fatalf("migration unexpected: %+v", result.Migration)
	}
	loaded, err := db.GetImageHostSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PublicBaseURL != "https://img.example.com" {
		t.Fatalf("settings not saved: %+v", loaded)
	}
}

func TestSaveSettingsWithOptionsRejectsInvalidAction(t *testing.T) {
	service, db, baseDir := newImageHostTestService(t)
	oldDir := filepath.Join(baseDir, "old")
	newDir := filepath.Join(baseDir, "new")
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = oldDir
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	settings.StorageDir = newDir
	if _, err := service.SaveSettingsWithOptions(settings, SaveSettingsOptions{StorageDirAction: "bad"}); err == nil {
		t.Fatal("expected invalid action error")
	}
	loaded, err := db.GetImageHostSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StorageDir != oldDir {
		t.Fatalf("settings should not be changed: %+v", loaded)
	}
}

func TestSaveSettingsWithOptionsRejectsConflictingTargetFile(t *testing.T) {
	service, db, baseDir := newImageHostTestService(t)
	oldDir := filepath.Join(baseDir, "old")
	newDir := filepath.Join(baseDir, "new")
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = oldDir
	if err := db.SaveImageHostSettings(settings); err != nil {
		t.Fatal(err)
	}
	asset, err := service.Upload(UploadInput{Reader: bytes.NewReader(testPNG(t)), OriginalName: "demo.png"})
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, asset.StorageKey), []byte("conflict"), 0644); err != nil {
		t.Fatal(err)
	}

	settings.StorageDir = newDir
	if _, err := service.SaveSettingsWithOptions(settings, SaveSettingsOptions{StorageDirAction: StorageDirActionMigrateDeleteOld}); err == nil {
		t.Fatal("expected target conflict error")
	}
	loaded, err := db.GetImageHostSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.StorageDir != oldDir {
		t.Fatalf("settings should not be changed: %+v", loaded)
	}
	if _, err := os.Stat(filepath.Join(oldDir, asset.StorageKey)); err != nil {
		t.Fatalf("old file should remain: %v", err)
	}
	if _, err := os.Stat(oldDir); err != nil {
		t.Fatalf("old dir should remain: %v", err)
	}
}

func newImageHostTestService(t *testing.T) (*Service, *config.Database, string) {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db), db, t.TempDir()
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testImage(), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testGIF(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, testImage(), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: uint8(x * 80), B: uint8(y * 60), A: 255})
		}
	}
	return img
}

func testWebP() []byte {
	return []byte("RIFF\x1a\x00\x00\x00WEBPVP8 \x0e\x00\x00\x00\x10\x00\x00\x00\x9d\x01\x2a\x01\x00\x01\x00\x00\x00\x25\xa0\x02\x74\x01\x00")
}
