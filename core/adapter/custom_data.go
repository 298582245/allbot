package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const CustomAdapterDataFile = "adapter.data"

var customAdapterDataLocks sync.Map

const customAdapterSystemConfigKey = "system_config"

type customAdapterDataDocument struct {
	SystemConfig map[string]json.RawMessage `json:"system_config"`
	Instances    map[string]json.RawMessage `json:"instances"`
}

// LoadCustomAdapterData reads per-instance config stored beside adapter.json.
func LoadCustomAdapterData(adapterDir string) (map[string]json.RawMessage, error) {
	document, err := loadCustomAdapterDataDocument(adapterDir)
	return document.Instances, err
}

// SaveCustomAdapterData atomically writes one instance's config to adapter.data.
func SaveCustomAdapterData(adapterDir, instanceID string, config interface{}) error {
	lockValue, _ := customAdapterDataLocks.LoadOrStore(filepath.Clean(adapterDir), &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	document, err := loadCustomAdapterDataDocument(adapterDir)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化适配器配置失败: %w", err)
	}
	document.Instances[instanceID] = encoded
	return saveCustomAdapterDataDocument(adapterDir, document)
}

// DeleteCustomAdapterData removes one instance's config while preserving others.
func DeleteCustomAdapterData(adapterDir, instanceID string) error {
	lockValue, _ := customAdapterDataLocks.LoadOrStore(filepath.Clean(adapterDir), &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	document, err := loadCustomAdapterDataDocument(adapterDir)
	if err != nil {
		return err
	}
	delete(document.Instances, instanceID)
	return saveCustomAdapterDataDocument(adapterDir, document)
}

func LoadCustomAdapterRuntimeProfile(adapterDir string) (string, error) {
	document, err := loadCustomAdapterDataDocument(adapterDir)
	if err != nil {
		return "", err
	}
	var profileID string
	if raw := document.SystemConfig["runtime_profile"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &profileID); err != nil {
			return "", fmt.Errorf("解析 %s 中的运行环境配置失败: %w", CustomAdapterDataFile, err)
		}
	}
	return strings.TrimSpace(profileID), nil
}

func SaveCustomAdapterRuntimeProfile(adapterDir, profileID string) error {
	profileID = strings.TrimSpace(profileID)
	lockValue, _ := customAdapterDataLocks.LoadOrStore(filepath.Clean(adapterDir), &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	document, err := loadCustomAdapterDataDocument(adapterDir)
	if err != nil {
		return err
	}
	if profileID == "" {
		delete(document.SystemConfig, "runtime_profile")
	} else {
		encoded, err := json.Marshal(strings.TrimSpace(profileID))
		if err != nil {
			return err
		}
		document.SystemConfig["runtime_profile"] = encoded
	}
	return saveCustomAdapterDataDocument(adapterDir, document)
}

func loadCustomAdapterDataDocument(adapterDir string) (customAdapterDataDocument, error) {
	path := filepath.Join(adapterDir, CustomAdapterDataFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newCustomAdapterDataDocument(), nil
	}
	if err != nil {
		return customAdapterDataDocument{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return customAdapterDataDocument{}, fmt.Errorf("解析 %s 失败: %w", CustomAdapterDataFile, err)
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	var document customAdapterDataDocument
	if _, structured := raw["instances"]; structured {
		if err := json.Unmarshal(data, &document); err != nil {
			return customAdapterDataDocument{}, fmt.Errorf("解析 %s 失败: %w", CustomAdapterDataFile, err)
		}
	} else if _, structured := raw[customAdapterSystemConfigKey]; structured {
		if err := json.Unmarshal(raw[customAdapterSystemConfigKey], &document.SystemConfig); err != nil {
			return customAdapterDataDocument{}, fmt.Errorf("解析 %s 的系统配置失败: %w", CustomAdapterDataFile, err)
		}
		delete(raw, customAdapterSystemConfigKey)
		document.Instances = raw
	} else {
		document.Instances = raw
	}
	if document.SystemConfig == nil {
		document.SystemConfig = map[string]json.RawMessage{}
	}
	if document.Instances == nil {
		document.Instances = map[string]json.RawMessage{}
	}
	return document, nil
}

func newCustomAdapterDataDocument() customAdapterDataDocument {
	return customAdapterDataDocument{SystemConfig: map[string]json.RawMessage{}, Instances: map[string]json.RawMessage{}}
}

func saveCustomAdapterDataDocument(adapterDir string, document customAdapterDataDocument) error {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(adapterDir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(adapterDir, ".adapter.data-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	target := filepath.Join(adapterDir, CustomAdapterDataFile)
	if err := os.Rename(tmpName, target); err != nil {
		if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(tmpName, target)
	}
	return nil
}
