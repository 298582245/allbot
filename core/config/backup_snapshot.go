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
	done := d.db.beginOperation()
	defer done()
	return d.snapshotTo(targetPath)
}

func (d *Database) snapshotTo(targetPath string) error {
	if d.db == nil || d.db.DB == nil {
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
	_, err = d.db.DB.Exec(`VACUUM INTO ?`, absPath)
	return err
}
