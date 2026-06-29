package config

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ScriptEnvVar struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	Remark    string    `json:"remark"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScriptEnvQuery struct {
	Keyword string
	Enabled *bool
	Names   []string
}

func (d *Database) SaveScriptEnvVar(item *ScriptEnvVar) (*ScriptEnvVar, error) {
	if err := normalizeScriptEnvVar(item); err != nil {
		return nil, err
	}
	now := time.Now()
	if item.ID == 0 {
		result, err := d.db.Exec(`
			INSERT INTO script_env_vars (name, value, remark, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, item.Name, item.Value, item.Remark, boolInt(item.Enabled), now, now)
		if err != nil {
			return nil, err
		}
		item.ID, _ = result.LastInsertId()
	} else {
		_, err := d.db.Exec(`
			UPDATE script_env_vars
			SET name = ?, value = ?, remark = ?, enabled = ?, updated_at = ?
			WHERE id = ?
		`, item.Name, item.Value, item.Remark, boolInt(item.Enabled), now, item.ID)
		if err != nil {
			return nil, err
		}
	}
	return d.GetScriptEnvVar(item.ID)
}

func (d *Database) GetScriptEnvVar(id int64) (*ScriptEnvVar, error) {
	row := d.db.QueryRow(`
		SELECT id, name, value, remark, enabled, created_at, updated_at
		FROM script_env_vars WHERE id = ?
	`, id)
	item, err := scanScriptEnvVar(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) ListScriptEnvVars(query ScriptEnvQuery) ([]*ScriptEnvVar, error) {
	conditions := []string{"1 = 1"}
	args := []interface{}{}
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		conditions = append(conditions, "(name LIKE ? OR remark LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	if query.Enabled != nil {
		conditions = append(conditions, "enabled = ?")
		args = append(args, boolInt(*query.Enabled))
	}
	if len(query.Names) > 0 {
		placeholders := make([]string, 0, len(query.Names))
		for _, name := range query.Names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, name)
		}
		if len(placeholders) > 0 {
			conditions = append(conditions, "name IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	rows, err := d.db.Query(`
		SELECT id, name, value, remark, enabled, created_at, updated_at
		FROM script_env_vars
		WHERE `+strings.Join(conditions, " AND ")+`
		ORDER BY name ASC, id ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*ScriptEnvVar, 0)
	for rows.Next() {
		item, err := scanScriptEnvVar(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) ScriptEnvMap(names []string) (map[string]string, error) {
	enabled := true
	items, err := d.ListScriptEnvVars(ScriptEnvQuery{Enabled: &enabled, Names: names})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(items))
	for _, item := range items {
		result[item.Name] = item.Value
	}
	return result, nil
}

func (d *Database) DeleteScriptEnvVar(id int64) error {
	_, err := d.db.Exec(`DELETE FROM script_env_vars WHERE id = ?`, id)
	return err
}

func normalizeScriptEnvVar(item *ScriptEnvVar) error {
	if item == nil {
		return fmt.Errorf("脚本环境变量不能为空")
	}
	item.Name = strings.TrimSpace(item.Name)
	item.Remark = strings.TrimSpace(item.Remark)
	if item.Name == "" || strings.ContainsAny(item.Name, "=\x00") {
		return fmt.Errorf("环境变量名无效")
	}
	return nil
}

type scriptEnvScanner interface {
	Scan(dest ...interface{}) error
}

func scanScriptEnvVar(scanner scriptEnvScanner) (*ScriptEnvVar, error) {
	var item ScriptEnvVar
	var enabled int
	if err := scanner.Scan(&item.ID, &item.Name, &item.Value, &item.Remark, &enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Enabled = enabled == 1
	return &item, nil
}
