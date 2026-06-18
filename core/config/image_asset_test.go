package config

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestImageHostSettingsDefaultAndSave(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	settings, err := db.GetImageHostSettings()
	if err != nil {
		t.Fatalf("GetImageHostSettings returned error: %v", err)
	}
	if settings.StorageDir != "./runtime/image_assets" || settings.MaxSize != 5*1024*1024 || len(settings.AllowedTypes) != 4 {
		t.Fatalf("default settings unexpected: %+v", settings)
	}

	saved := ImageHostSettings{StorageDir: " ./runtime/custom ", PublicBaseURL: "https://example.com/", MaxSize: 1024, AllowedTypes: []string{"IMAGE/PNG", "image/png", " image/jpeg "}}
	if err := db.SaveImageHostSettings(saved); err != nil {
		t.Fatalf("SaveImageHostSettings returned error: %v", err)
	}
	loaded, err := db.GetImageHostSettings()
	if err != nil {
		t.Fatalf("GetImageHostSettings returned error: %v", err)
	}
	if loaded.StorageDir != "./runtime/custom" || loaded.PublicBaseURL != "https://example.com" || loaded.MaxSize != 1024 {
		t.Fatalf("saved settings unexpected: %+v", loaded)
	}
	if len(loaded.AllowedTypes) != 2 || loaded.AllowedTypes[0] != "image/png" || loaded.AllowedTypes[1] != "image/jpeg" {
		t.Fatalf("allowed types unexpected: %#v", loaded.AllowedTypes)
	}
}

func TestSaveImageHostSettingsRejectsInvalid(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.SaveImageHostSettings(ImageHostSettings{StorageDir: " ", MaxSize: 1, AllowedTypes: []string{"image/png"}}); err == nil {
		t.Fatal("expected blank storage dir error")
	}
	if err := db.SaveImageHostSettings(ImageHostSettings{StorageDir: "./runtime/images", MaxSize: 0, AllowedTypes: []string{"image/png"}}); err == nil {
		t.Fatal("expected invalid max size error")
	}
	if err := db.SaveImageHostSettings(ImageHostSettings{StorageDir: "./runtime/images", MaxSize: 1, AllowedTypes: []string{" "}}); err == nil {
		t.Fatal("expected blank allowed types error")
	}
}

func TestImageAssetCRUD(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	created, err := db.CreateImageAsset(&ImageAsset{PublicID: "pub1", OriginalName: "demo.png", StorageKey: "pub1.png", Ext: "png", ContentType: "image/png", SizeBytes: 12, Width: 2, Height: 3, SHA256: "abc"})
	if err != nil {
		t.Fatalf("CreateImageAsset returned error: %v", err)
	}
	if created.ID == 0 || created.PublicID != "pub1" || created.Width != 2 || created.Height != 3 {
		t.Fatalf("created asset unexpected: %+v", created)
	}

	got, err := db.GetImageAssetByPublicID(" pub1 ")
	if err != nil {
		t.Fatalf("GetImageAssetByPublicID returned error: %v", err)
	}
	if got.OriginalName != "demo.png" || got.ContentType != "image/png" {
		t.Fatalf("got asset unexpected: %+v", got)
	}

	items, total, err := db.ListImageAssets(ImageAssetQuery{Keyword: "demo", ContentType: "image/png", Limit: 10})
	if err != nil {
		t.Fatalf("ListImageAssets returned error: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].PublicID != "pub1" {
		t.Fatalf("list unexpected: total=%d items=%+v", total, items)
	}

	if err := db.DeleteImageAsset("pub1"); err != nil {
		t.Fatalf("DeleteImageAsset returned error: %v", err)
	}
	if _, err := db.GetImageAssetByPublicID("pub1"); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestCreateImageAssetRejectsDuplicatePublicID(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	asset := &ImageAsset{PublicID: "dup", OriginalName: "demo.png", StorageKey: "dup.png", Ext: "png", ContentType: "image/png", SizeBytes: 12}
	if _, err := db.CreateImageAsset(asset); err != nil {
		t.Fatalf("CreateImageAsset returned error: %v", err)
	}
	if _, err := db.CreateImageAsset(asset); err == nil {
		t.Fatal("expected duplicate public_id error")
	}
}

func TestListAllImageAssetsIgnoresPaginationLimit(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 205; i++ {
		publicID := fmt.Sprintf("pub%03d", i)
		if _, err := db.CreateImageAsset(&ImageAsset{PublicID: publicID, OriginalName: publicID + ".png", StorageKey: publicID + ".png", Ext: "png", ContentType: "image/png", SizeBytes: 12}); err != nil {
			t.Fatalf("CreateImageAsset returned error: %v", err)
		}
	}
	paged, total, err := db.ListImageAssets(ImageAssetQuery{Limit: 500})
	if err != nil {
		t.Fatalf("ListImageAssets returned error: %v", err)
	}
	if total != 205 || len(paged) != 200 {
		t.Fatalf("paged list unexpected: total=%d len=%d", total, len(paged))
	}
	all, err := db.ListAllImageAssets()
	if err != nil {
		t.Fatalf("ListAllImageAssets returned error: %v", err)
	}
	if len(all) != 205 {
		t.Fatalf("expected all assets, got %d", len(all))
	}
}
