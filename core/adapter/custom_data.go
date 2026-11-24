package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const CustomAdapterDataFile = "adapter.data"

var customAdapterDataLocks sync.Map

// LoadCustomAdapterData reads per-instance config stored beside adapter.json.
func LoadCustomAdapterData(adapterDir string) (map[string]json.RawMessage, error) {
	path := filepath.Join(adapterDir, CustomAdapterDataFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", CustomAdapterDataFile, err)
	}
	if result == nil {
		result = map[string]json.RawMessage{}
	}
	return result, nil
}

// SaveCustomAdapterData atomically writes one instance's config to adapter.data.
func SaveCustomAdapterData(adapterDir, instanceID string, config interface{}) error {
	lockValue, _ := customAdapterDataLocks.LoadOrStore(filepath.Clean(adapterDir), &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	configs, err := LoadCustomAdapterData(adapterDir)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化适配器配置失败: %w", err)
	}
	configs[instanceID] = encoded
	data, err := json.MarshalIndent(configs, "", "  ")
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
		// Windows does not replace an existing destination with Rename.
		if removeErr := os.Remove(target); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(tmpName, target)
	}
	return nil
}

// DeleteCustomAdapterData removes one instance's config while preserving others.
func DeleteCustomAdapterData(adapterDir, instanceID string) error {
	lockValue, _ := customAdapterDataLocks.LoadOrStore(filepath.Clean(adapterDir), &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	configs, err := LoadCustomAdapterData(adapterDir)
	if err != nil {
		return err
	}
	delete(configs, instanceID)
	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
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
