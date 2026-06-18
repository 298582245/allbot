package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func (d *Database) SnapshotTo(targetPath string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, err = d.db.Exec(`VACUUM INTO ?`, absPath)
	return err
}
