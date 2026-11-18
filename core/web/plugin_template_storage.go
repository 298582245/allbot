package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
)

const qinglongDataFileName = "qinglong.data"

func (s *Server) templateMetadataPath(pluginID string) (string, error) {
	if s.pluginManager == nil {
		return "", fmt.Errorf("插件管理器不可用")
	}
	return filepath.Join(s.pluginManager.PluginPath(pluginID), qinglongDataFileName), nil
}

func (s *Server) saveTemplateMetadata(item *config.PluginTemplateMetadata) error {
	if item == nil {
		return fmt.Errorf("插件模板元数据不能为空")
	}
	item.PluginID = strings.TrimSpace(item.PluginID)
	item.Template = strings.TrimSpace(item.Template)
	item.TemplateVersion = strings.TrimSpace(item.TemplateVersion)
	item.Runtime = strings.TrimSpace(item.Runtime)
	item.Structure = strings.TrimSpace(item.Structure)
	if item.PluginID == "" || item.Template == "" || item.TemplateVersion == "" || item.Runtime == "" {
		return fmt.Errorf("插件模板元数据缺少必要字段")
	}
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	if item.TemplateSource == nil {
		item.TemplateSource = map[string]interface{}{}
	}
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now

	path, err := s.templateMetadataPath(item.PluginID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	return writePluginMetadataFile(path, data)
}

func writePluginMetadataFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".qinglong-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}

	backupPath := path + ".allbot-old"
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(path, backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		if restoreErr := os.Rename(backupPath, path); restoreErr != nil && !os.IsNotExist(restoreErr) {
			return errors.Join(err, restoreErr)
		}
		return err
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Server) deleteTemplateMetadata(pluginID string) error {
	var fileErr error
	if path, err := s.templateMetadataPath(pluginID); err != nil {
		fileErr = err
	} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fileErr = err
	}
	var databaseErr error
	if database := s.pluginTemplateMetadataDatabase(); database != nil {
		databaseErr = database.DeletePluginTemplateMetadata(pluginID)
	}
	return errors.Join(fileErr, databaseErr)
}
