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
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScriptEnvQuery struct {
	Keyword string
	Enabled *bool
	Names   []string
}

type ScriptEnvImportItem struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Remark  string `json:"remark"`
	Enabled *bool  `json:"enabled,omitempty"`
	Pinned  *bool  `json:"pinned,omitempty"`
}

func (d *Database) SaveScriptEnvVar(item *ScriptEnvVar) (*ScriptEnvVar, error) {
	if err := normalizeScriptEnvVar(item); err != nil {
		return nil, err
	}
	if err := d.ensureScriptEnvNameValueUnique(item.ID, item.Name, item.Value); err != nil {
		return nil, err
	}
	now := time.Now()
	if item.ID == 0 {
		result, err := d.db.Exec(`
			INSERT INTO script_env_vars (name, value, remark, enabled, pinned, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, item.Name, item.Value, item.Remark, boolInt(item.Enabled), boolInt(item.Pinned), now, now)
		if err != nil {
			return nil, formatScriptEnvDuplicateError(err)
		}
		item.ID, _ = result.LastInsertId()
	} else {
		_, err := d.db.Exec(`
			UPDATE script_env_vars
			SET name = ?, value = ?, remark = ?, enabled = ?, pinned = ?, updated_at = ?
			WHERE id = ?
		`, item.Name, item.Value, item.Remark, boolInt(item.Enabled), boolInt(item.Pinned), now, item.ID)
		if err != nil {
			return nil, formatScriptEnvDuplicateError(err)
		}
	}
	return d.GetScriptEnvVar(item.ID)
}

func (d *Database) GetScriptEnvVar(id int64) (*ScriptEnvVar, error) {
	row := d.db.QueryRow(`
		SELECT id, name, value, remark, enabled, pinned, created_at, updated_at
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
		SELECT id, name, value, remark, enabled, pinned, created_at, updated_at
		FROM script_env_vars
		WHERE `+strings.Join(conditions, " AND ")+`
		ORDER BY pinned DESC, id DESC
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
	values := make(map[string][]string, len(items))
	for _, item := range items {
		values[item.Name] = append(values[item.Name], item.Value)
	}
	result := make(map[string]string, len(values))
	for name, itemValues := range values {
		result[name] = strings.Join(itemValues, "&")
	}
	return result, nil
}

func (d *Database) DeleteScriptEnvVar(id int64) error {
	_, err := d.db.Exec(`DELETE FROM script_env_vars WHERE id = ?`, id)
	return err
}

func (d *Database) DeleteScriptEnvVars(ids []int64) (int64, error) {
	return d.execScriptEnvIDs(`DELETE FROM script_env_vars WHERE id IN (%s)`, ids)
}

func (d *Database) UpdateScriptEnvVarsEnabled(ids []int64, enabled bool) (int64, error) {
	return d.execScriptEnvIDs(`UPDATE script_env_vars SET enabled = ?, updated_at = ? WHERE id IN (%s)`, ids, boolInt(enabled), time.Now())
}

func (d *Database) UpdateScriptEnvVarsPinned(ids []int64, pinned bool) (int64, error) {
	return d.execScriptEnvIDs(`UPDATE script_env_vars SET pinned = ?, updated_at = ? WHERE id IN (%s)`, ids, boolInt(pinned), time.Now())
}

func (d *Database) ImportScriptEnvVars(importItems []ScriptEnvImportItem) (int64, error) {
	items := make([]ScriptEnvVar, 0, len(importItems))
	seen := make(map[string]bool, len(importItems))
	for i, importItem := range importItems {
		item := ScriptEnvVar{Name: importItem.Name, Value: importItem.Value, Remark: importItem.Remark, Enabled: true}
		if importItem.Enabled != nil {
			item.Enabled = *importItem.Enabled
		}
		if importItem.Pinned != nil {
			item.Pinned = *importItem.Pinned
		}
		if err := normalizeScriptEnvVar(&item); err != nil {
			return 0, fmt.Errorf("第 %d 个变量无效: %w", i+1, err)
		}
		key := scriptEnvNameValueKey(item.Name, item.Value)
		if seen[key] {
			return 0, fmt.Errorf("导入文件中存在重复变量名和值: %s", item.Name)
		}
		seen[key] = true
		if err := d.ensureScriptEnvNameValueUnique(0, item.Name, item.Value); err != nil {
			return 0, err
		}
		items = append(items, item)
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now()
	for _, item := range items {
		if _, err := tx.Exec(`
			INSERT INTO script_env_vars (name, value, remark, enabled, pinned, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, item.Name, item.Value, item.Remark, boolInt(item.Enabled), boolInt(item.Pinned), now, now); err != nil {
			return 0, formatScriptEnvDuplicateError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}

func (d *Database) execScriptEnvIDs(queryTemplate string, ids []int64, prefixArgs ...interface{}) (int64, error) {
	ids = normalizeIDs(ids)
	if len(ids) == 0 {
		return 0, fmt.Errorf("请选择要操作的脚本环境变量")
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(prefixArgs)+len(ids))
	args = append(args, prefixArgs...)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	result, err := d.db.Exec(fmt.Sprintf(queryTemplate, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func normalizeIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func (d *Database) ensureScriptEnvNameValueUnique(id int64, name, value string) error {
	var existingID int64
	err := d.db.QueryRow(`
		SELECT id FROM script_env_vars
		WHERE name = ? AND value = ? AND id <> ?
		LIMIT 1
	`, name, value, id).Scan(&existingID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("已存在相同变量名和值的脚本环境变量")
}

func formatScriptEnvDuplicateError(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		return fmt.Errorf("已存在相同变量名和值的脚本环境变量")
	}
	return err
}

func scriptEnvNameValueKey(name, value string) string {
	return name + "\x00" + value
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
	var pinned int
	if err := scanner.Scan(&item.ID, &item.Name, &item.Value, &item.Remark, &enabled, &pinned, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Enabled = enabled == 1
	item.Pinned = pinned == 1
	return &item, nil
}
