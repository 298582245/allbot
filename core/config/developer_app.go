package config

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type DeveloperApp struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	ClientID          string     `json:"appKey"`
	SecretHash        string     `json:"-"`
	Permissions       []string   `json:"permissions"`
	CredentialVersion int64      `json:"-"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
}

func (d *Database) CreateDeveloperApp(app DeveloperApp) error {
	permissions, err := json.Marshal(app.Permissions)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`INSERT INTO developer_apps(id,name,client_id,client_secret_hash,permissions,credential_version) VALUES(?,?,?,?,?,1)`, app.ID, app.Name, app.ClientID, app.SecretHash, string(permissions))
	if err != nil {
		return err
	}
	return d.auditDeveloperApp(app.ID, "created")
}

func (d *Database) ListDeveloperApps(keyword string, page, pageSize int) ([]DeveloperApp, int64, int, error) {
	pageSize, _ = normalizeLimitOffset(pageSize, 0)
	if page < 1 {
		page = 1
	}
	keyword = strings.TrimSpace(keyword)
	where := ` WHERE deleted_at IS NULL`
	var args []any
	if keyword != "" {
		like := "%" + escapeLike(keyword) + "%"
		where += ` AND (name LIKE ? ESCAPE '\' OR client_id LIKE ? ESCAPE '\')`
		args = append(args, like, like)
	}
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM developer_apps`+where, args...).Scan(&total); err != nil {
		return nil, 0, page, err
	}
	if total == 0 {
		page = 1
	} else if maxPage := int((total + int64(pageSize) - 1) / int64(pageSize)); page > maxPage {
		page = maxPage
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any(nil), args...), pageSize, offset)
	rows, err := d.db.Query(`SELECT id,name,client_id,client_secret_hash,permissions,credential_version,created_at,updated_at,last_used_at FROM developer_apps`+where+` ORDER BY created_at DESC,id LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, page, err
	}
	defer rows.Close()
	apps := make([]DeveloperApp, 0)
	for rows.Next() {
		var app DeveloperApp
		var rawPermissions string
		var lastUsed sql.NullTime
		if err := rows.Scan(&app.ID, &app.Name, &app.ClientID, &app.SecretHash, &rawPermissions, &app.CredentialVersion, &app.CreatedAt, &app.UpdatedAt, &lastUsed); err != nil {
			return nil, 0, page, err
		}
		if err := json.Unmarshal([]byte(rawPermissions), &app.Permissions); err != nil {
			return nil, 0, page, err
		}
		if lastUsed.Valid {
			app.LastUsedAt = &lastUsed.Time
		}
		app.Status = "active"
		apps = append(apps, app)
	}
	return apps, total, page, rows.Err()
}

func (d *Database) GetDeveloperAppByClientID(clientID string) (*DeveloperApp, error) {
	var app DeveloperApp
	var rawPermissions string
	var lastUsed sql.NullTime
	err := d.db.QueryRow(`SELECT id,name,client_id,client_secret_hash,permissions,credential_version,created_at,updated_at,last_used_at FROM developer_apps WHERE client_id=? AND deleted_at IS NULL`, clientID).Scan(&app.ID, &app.Name, &app.ClientID, &app.SecretHash, &rawPermissions, &app.CredentialVersion, &app.CreatedAt, &app.UpdatedAt, &lastUsed)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(rawPermissions), &app.Permissions); err != nil {
		return nil, err
	}
	if lastUsed.Valid {
		app.LastUsedAt = &lastUsed.Time
	}
	app.Status = "active"
	return &app, nil
}

func (d *Database) UpdateDeveloperApp(appID, name string, permissions []string) error {
	raw, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	result, err := d.db.Exec(`UPDATE developer_apps SET name=?,permissions=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND deleted_at IS NULL`, name, string(raw), appID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return d.auditDeveloperApp(appID, "updated")
}

func (d *Database) ResetDeveloperAppSecret(appID, secretHash string) (int64, error) {
	result, err := d.db.Exec(`UPDATE developer_apps SET client_secret_hash=?,credential_version=credential_version+1,updated_at=CURRENT_TIMESTAMP WHERE id=? AND deleted_at IS NULL`, secretHash, appID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, sql.ErrNoRows
	}
	app, err := d.GetDeveloperAppByID(appID)
	if err != nil {
		return 0, err
	}
	if err := d.auditDeveloperApp(appID, "secret_reset"); err != nil {
		return 0, err
	}
	return app.CredentialVersion, nil
}

func (d *Database) DeleteDeveloperApp(appID string) error {
	result, err := d.db.Exec(`UPDATE developer_apps SET deleted_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=? AND deleted_at IS NULL`, appID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return d.auditDeveloperApp(appID, "deleted")
}

func (d *Database) GetDeveloperAppByID(appID string) (*DeveloperApp, error) {
	var app DeveloperApp
	var rawPermissions string
	err := d.db.QueryRow(`SELECT id,name,client_id,client_secret_hash,permissions,credential_version,created_at,updated_at FROM developer_apps WHERE id=? AND deleted_at IS NULL`, appID).Scan(&app.ID, &app.Name, &app.ClientID, &app.SecretHash, &rawPermissions, &app.CredentialVersion, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(rawPermissions), &app.Permissions); err != nil {
		return nil, err
	}
	app.Status = "active"
	return &app, nil
}

func (d *Database) TouchDeveloperApp(appID string) error {
	_, err := d.db.Exec(`UPDATE developer_apps SET last_used_at=CURRENT_TIMESTAMP WHERE id=? AND deleted_at IS NULL`, appID)
	return err
}

func (d *Database) auditDeveloperApp(appID, action string) error {
	_, err := d.db.Exec(`INSERT INTO developer_app_audit(app_id,action) VALUES(?,?)`, appID, action)
	return err
}

func (d *Database) RecordDeveloperAppAudit(appID, action, actor string) error {
	_, err := d.db.Exec(`INSERT INTO developer_app_audit(app_id,action,actor) VALUES(?,?,?)`, appID, action, actor)
	return err
}

func IsDeveloperAppNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
