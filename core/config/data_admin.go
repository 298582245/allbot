package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type TableInfo struct {
	Name        string       `json:"name"`
	Count       int          `json:"count"`
	Group       string       `json:"group"`
	ViewName    string       `json:"view_name"`
	Description string       `json:"description"`
	PluginID    string       `json:"plugin_id"`
	Cols        []ColumnInfo `json:"columns"`
}

type ColumnInfo struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	NotNull    bool        `json:"not_null"`
	Default    interface{} `json:"default"`
	PrimaryKey bool        `json:"primary_key"`
}

type TableRows struct {
	Table   string                   `json:"table"`
	Columns []ColumnInfo             `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Total   int                      `json:"total"`
	Page    int                      `json:"page"`
	Size    int                      `json:"size"`
}

type ExportData struct {
	Version string                   `json:"version"`
	Tables  []TableRows              `json:"tables"`
	Rows    []map[string]interface{} `json:"rows,omitempty"`
}

type DataViewConfig struct {
	PluginID    string   `json:"plugin_id"`
	TableName   string   `json:"table_name"`
	ViewName    string   `json:"view_name"`
	GroupName   string   `json:"group_name"`
	Description string   `json:"description"`
	Columns     []string `json:"columns"`
}

type PluginTableColumn struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
}

type PluginDBFilter struct {
	Field    string        `json:"field"`
	Op       string        `json:"op"`
	Operator string        `json:"operator"`
	Value    interface{}   `json:"value"`
	Values   []interface{} `json:"values"`
}

type PluginDBQuery struct {
	Table    string           `json:"table"`
	Where    string           `json:"where"`
	Args     []interface{}    `json:"args"`
	Filters  []PluginDBFilter `json:"filters"`
	Order    interface{}      `json:"order"`
	OrderBy  string           `json:"order_by"`
	OrderDir string           `json:"order_dir"`
	Limit    int              `json:"limit"`
	Page     int              `json:"page"`
	Size     int              `json:"size"`
}

var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func isDataAdminHiddenTable(table string) bool {
	return strings.EqualFold(strings.TrimSpace(table), "adapters")
}

func (d *Database) EnsurePluginTable(pluginID, table string, columns []PluginTableColumn) (string, error) {
	tableName, err := pluginTableName(pluginID, table)
	if err != nil {
		return "", err
	}

	columnDefs := []string{`id INTEGER PRIMARY KEY AUTOINCREMENT`}
	seen := map[string]bool{"id": true, "created_at": true, "updated_at": true}
	for _, column := range columns {
		name := strings.TrimSpace(column.Name)
		if !sqlIdentifierPattern.MatchString(name) {
			return "", fmt.Errorf("字段名无效: %s", name)
		}
		lowerName := strings.ToLower(name)
		if seen[lowerName] {
			continue
		}
		seen[lowerName] = true
		columnDefs = append(columnDefs, fmt.Sprintf(`%s %s`, quoteIdentifier(name), normalizePluginColumnType(column.Type)))
	}
	columnDefs = append(columnDefs, `created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`, `updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`)

	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s)`, quoteIdentifier(tableName), strings.Join(columnDefs, ", "))
	if _, err := d.db.Exec(query); err != nil {
		return "", err
	}
	if err := d.ensurePluginTableColumns(tableName, columns); err != nil {
		return "", err
	}
	return tableName, nil
}

func (d *Database) ensurePluginTableColumns(tableName string, columns []PluginTableColumn) error {
	existingColumns, err := d.TableColumns(tableName)
	if err != nil {
		return err
	}
	exists := make(map[string]bool, len(existingColumns))
	for _, column := range existingColumns {
		exists[strings.ToLower(column.Name)] = true
	}
	for _, column := range columns {
		name := strings.TrimSpace(column.Name)
		if name == "" || exists[strings.ToLower(name)] {
			continue
		}
		if !sqlIdentifierPattern.MatchString(name) {
			return fmt.Errorf("字段名无效: %s", name)
		}
		_, err := d.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, quoteIdentifier(tableName), quoteIdentifier(name), normalizePluginColumnType(column.Type)))
		if err != nil {
			return err
		}
		exists[strings.ToLower(name)] = true
	}
	return nil
}

func (d *Database) QueryPluginRows(pluginID string, query PluginDBQuery) (*TableRows, error) {
	tableName, err := pluginTableName(pluginID, query.Table)
	if err != nil {
		return nil, err
	}
	if err := d.ensureTableName(tableName); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.Where) != "" || len(query.Filters) > 0 || pluginOrderProvided(query.Order) || strings.TrimSpace(query.OrderBy) != "" || query.Limit > 0 {
		return d.queryPluginRowsByCondition(tableName, query)
	}
	return d.QueryTableRows(tableName, query.Page, query.Size, "")
}

func (d *Database) InsertPluginRow(pluginID, table string, values map[string]interface{}) (int64, error) {
	tableName, err := pluginTableName(pluginID, table)
	if err != nil {
		return 0, err
	}
	return d.insertRowWithID(tableName, values)
}

func (d *Database) UpdatePluginRow(pluginID, table string, rowID int64, values map[string]interface{}) error {
	tableName, err := pluginTableName(pluginID, table)
	if err != nil {
		return err
	}
	return d.UpdateTableRow(tableName, rowID, values)
}

func (d *Database) DeletePluginRow(pluginID, table string, rowID int64) error {
	tableName, err := pluginTableName(pluginID, table)
	if err != nil {
		return err
	}
	return d.DeleteTableRow(tableName, rowID)
}

func (d *Database) ClearPluginTable(pluginID, table string) error {
	tableName, err := pluginTableName(pluginID, table)
	if err != nil {
		return err
	}
	return d.ClearTable(tableName)
}

func (d *Database) ListTables() ([]TableInfo, error) {
	rows, err := d.db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	views, err := d.listDataViews()
	if err != nil {
		return nil, err
	}

	var result []TableInfo
	for _, name := range names {
		if isDataAdminHiddenTable(name) {
			continue
		}
		columns, err := d.TableColumns(name)
		if err != nil {
			return nil, err
		}
		count, err := d.tableRowCount(name)
		if err != nil {
			return nil, err
		}
		view := views[name]
		group := tableGroup(name)
		viewName := name
		if view.TableName != "" {
			group = view.GroupName
			viewName = view.ViewName
		}
		result = append(result, TableInfo{Name: name, Count: count, Group: group, ViewName: viewName, Description: view.Description, PluginID: view.PluginID, Cols: columns})
	}
	return result, nil
}

func (d *Database) SaveDataView(view DataViewConfig) error {
	if err := d.ensureDataAdminTableName(view.TableName); err != nil {
		return err
	}
	if view.ViewName == "" {
		view.ViewName = view.TableName
	}
	if view.GroupName == "" {
		view.GroupName = tableGroup(view.TableName)
	}
	columns, err := json.Marshal(view.Columns)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`
		INSERT INTO data_views (plugin_id, table_name, view_name, group_name, description, columns, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(plugin_id, table_name) DO UPDATE SET
			view_name = excluded.view_name,
			group_name = excluded.group_name,
			description = excluded.description,
			columns = excluded.columns,
			updated_at = CURRENT_TIMESTAMP
	`, view.PluginID, view.TableName, view.ViewName, view.GroupName, view.Description, string(columns))
	return err
}

func (d *Database) listDataViews() (map[string]DataViewConfig, error) {
	rows, err := d.db.Query(`SELECT plugin_id, table_name, view_name, group_name, description, columns FROM data_views`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]DataViewConfig)
	for rows.Next() {
		var view DataViewConfig
		var columnsJSON string
		if err := rows.Scan(&view.PluginID, &view.TableName, &view.ViewName, &view.GroupName, &view.Description, &columnsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(columnsJSON), &view.Columns)
		if _, exists := result[view.TableName]; !exists || view.PluginID != "" {
			result[view.TableName] = view
		}
	}
	return result, rows.Err()
}

func (d *Database) RenameTable(oldName, newName string) error {
	if err := d.ensureDataAdminTableName(oldName); err != nil {
		return err
	}
	if isDataAdminHiddenTable(newName) {
		return fmt.Errorf("新表名无效")
	}
	if !sqlIdentifierPattern.MatchString(newName) {
		return fmt.Errorf("新表名无效")
	}
	var exists string
	err := d.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, newName).Scan(&exists)
	if err == nil {
		return fmt.Errorf("表已存在: %s", newName)
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = d.db.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, quoteIdentifier(oldName), quoteIdentifier(newName)))
	return err
}

func (d *Database) ClearTable(table string) error {
	if err := d.ensureDataAdminTableName(table); err != nil {
		return err
	}
	_, err := d.db.Exec(fmt.Sprintf(`DELETE FROM %s`, quoteIdentifier(table)))
	return err
}

func (d *Database) DropTable(table string) error {
	if err := d.ensureDataAdminTableName(table); err != nil {
		return err
	}
	_, err := d.db.Exec(fmt.Sprintf(`DROP TABLE %s`, quoteIdentifier(table)))
	return err
}

func (d *Database) TableColumns(table string) ([]ColumnInfo, error) {
	if err := d.ensureTableName(table); err != nil {
		return nil, err
	}

	rows, err := d.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, quoteIdentifier(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, ColumnInfo{
			Name:       name,
			Type:       columnType,
			NotNull:    notNull == 1,
			Default:    normalizeDBValue(defaultValue),
			PrimaryKey: primaryKey > 0,
		})
	}
	return columns, rows.Err()
}

func (d *Database) QueryTableRows(table string, page, size int, search string) (*TableRows, error) {
	if err := d.ensureDataAdminTableName(table); err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 500 {
		size = 20
	}

	columns, err := d.TableColumns(table)
	if err != nil {
		return nil, err
	}
	search = strings.TrimSpace(search)
	where, args, err := d.dataAdminSearchWhere(table, columns, search)
	if err != nil {
		return nil, err
	}

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, quoteIdentifier(table), where)
	var total int
	if err := d.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	orderColumn := "rowid"
	for _, column := range columns {
		if column.PrimaryKey {
			orderColumn = quoteIdentifier(column.Name)
			break
		}
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, size, (page-1)*size)
	query := fmt.Sprintf(`SELECT rowid AS __rowid__, * FROM %s%s ORDER BY %s LIMIT ? OFFSET ?`, quoteIdentifier(table), where, orderColumn)
	rows, err := d.db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	return &TableRows{Table: table, Columns: columns, Rows: items, Total: total, Page: page, Size: size}, nil
}

func (d *Database) dataAdminSearchWhere(table string, columns []ColumnInfo, search string) (string, []interface{}, error) {
	if search == "" {
		return "", nil, nil
	}
	if matched, err := d.dataAdminTableMatchesSearch(table, search); err != nil {
		return "", nil, err
	} else if matched {
		return "", nil, nil
	}

	pattern := dataAdminLikePattern(search)
	clauses := []string{`CAST(rowid AS TEXT) LIKE ? ESCAPE '\'`}
	args := []interface{}{pattern}
	for _, column := range columns {
		clauses = append(clauses, fmt.Sprintf(`CAST(%s AS TEXT) LIKE ? ESCAPE '\'`, quoteIdentifier(column.Name)))
		args = append(args, pattern)
	}
	return " WHERE " + strings.Join(clauses, " OR "), args, nil
}

func (d *Database) dataAdminTableMatchesSearch(table string, search string) (bool, error) {
	keyword := strings.ToLower(search)
	views, err := d.listDataViews()
	if err != nil {
		return false, err
	}
	view := views[table]
	values := []string{table, tableGroup(table), view.ViewName, view.Description, view.PluginID}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true, nil
		}
	}
	return false, nil
}

func dataAdminLikePattern(search string) string {
	var builder strings.Builder
	builder.WriteByte('%')
	for _, r := range search {
		if r == '\\' || r == '%' || r == '_' {
			builder.WriteByte('\\')
		}
		builder.WriteRune(r)
	}
	builder.WriteByte('%')
	return builder.String()
}

func (d *Database) InsertTableRow(table string, values map[string]interface{}) error {
	if err := d.ensureDataAdminTableName(table); err != nil {
		return err
	}
	_, err := d.insertRowWithID(table, values)
	return err
}

func (d *Database) insertRowWithID(table string, values map[string]interface{}) (int64, error) {
	columns, err := d.editableColumns(table)
	if err != nil {
		return 0, err
	}
	if table == "data_views" {
		values = normalizeDataViewValues(values)
	}

	validValues := make(map[string]interface{})
	for _, column := range columns {
		if value, ok := values[column.Name]; ok {
			validValues[column.Name] = normalizeInputValue(value)
		}
	}
	if len(validValues) == 0 {
		return 0, fmt.Errorf("没有可写入的字段")
	}

	names := make([]string, 0, len(validValues))
	for name := range validValues {
		names = append(names, name)
	}
	sort.Strings(names)

	placeholders := make([]string, 0, len(names))
	args := make([]interface{}, 0, len(names))
	quotedNames := make([]string, 0, len(names))
	for _, name := range names {
		quotedNames = append(quotedNames, quoteIdentifier(name))
		placeholders = append(placeholders, "?")
		args = append(args, validValues[name])
	}

	query := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, quoteIdentifier(table), strings.Join(quotedNames, ", "), strings.Join(placeholders, ", "))
	result, err := d.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	if table == "data_views" {
		return id, d.normalizeDataViewsTable()
	}
	return id, nil
}

func (d *Database) UpdateTableRow(table string, rowID int64, values map[string]interface{}) error {
	if err := d.ensureDataAdminTableName(table); err != nil {
		return err
	}
	if rowID <= 0 {
		return fmt.Errorf("行 ID 无效")
	}
	columns, err := d.editableColumns(table)
	if err != nil {
		return err
	}
	if table == "data_views" {
		values = normalizeDataViewValues(values)
	}

	columnMap := make(map[string]ColumnInfo)
	for _, column := range columns {
		columnMap[column.Name] = column
	}

	names := make([]string, 0)
	for name := range values {
		if _, ok := columnMap[name]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("没有可更新的字段")
	}

	sets := make([]string, 0, len(names))
	args := make([]interface{}, 0, len(names)+1)
	for _, name := range names {
		sets = append(sets, fmt.Sprintf(`%s = ?`, quoteIdentifier(name)))
		args = append(args, normalizeInputValue(values[name]))
	}
	allColumns, _ := d.TableColumns(table)
	if hasEditableColumn(allColumns, "updated_at") {
		sets = append(sets, `updated_at = CURRENT_TIMESTAMP`)
	}
	args = append(args, rowID)

	query := fmt.Sprintf(`UPDATE %s SET %s WHERE rowid = ?`, quoteIdentifier(table), strings.Join(sets, ", "))
	result, err := d.db.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("数据不存在或未更新")
	}
	if table == "data_views" {
		return d.normalizeDataViewsTable()
	}
	return nil
}

func (d *Database) DeleteTableRow(table string, rowID int64) error {
	if err := d.ensureDataAdminTableName(table); err != nil {
		return err
	}
	if rowID <= 0 {
		return fmt.Errorf("行 ID 无效")
	}
	result, err := d.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE rowid = ?`, quoteIdentifier(table)), rowID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("数据不存在")
	}
	return nil
}

func (d *Database) queryPluginRowsByCondition(table string, query PluginDBQuery) (*TableRows, error) {
	columns, err := d.TableColumns(table)
	if err != nil {
		return nil, err
	}
	columnMap := pluginColumnMap(columns)
	where, args, err := buildPluginWhere(query.Where, query.Args, query.Filters, columnMap)
	if err != nil {
		return nil, err
	}
	order, err := buildPluginOrder(query.Order, query.OrderBy, query.OrderDir, columnMap)
	if err != nil {
		return nil, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = query.Size
	}
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	page := query.Page
	if page < 1 {
		page = 1
	}

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, quoteIdentifier(table), where)
	var total int
	if err := d.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	selectArgs := append([]interface{}{}, args...)
	selectArgs = append(selectArgs, limit, (page-1)*limit)
	selectSQL := fmt.Sprintf(`SELECT rowid AS __rowid__, * FROM %s%s%s LIMIT ? OFFSET ?`, quoteIdentifier(table), where, order)
	rows, err := d.db.Query(selectSQL, selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return &TableRows{Table: table, Columns: columns, Rows: items, Total: total, Page: page, Size: limit}, nil
}

func (d *Database) ExportTables(table string) (*ExportData, error) {
	tables := []string{}
	if table != "" {
		if isDataAdminHiddenTable(table) {
			return nil, fmt.Errorf("表不存在: %s", table)
		}
		if err := d.ensureTableName(table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	} else {
		infos, err := d.ListTables()
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			tables = append(tables, info.Name)
		}
	}

	exportData := &ExportData{Version: "1", Tables: make([]TableRows, 0, len(tables))}
	for _, tableName := range tables {
		columns, err := d.TableColumns(tableName)
		if err != nil {
			return nil, err
		}
		rows, err := d.selectAllRows(tableName)
		if err != nil {
			return nil, err
		}
		exportData.Tables = append(exportData.Tables, TableRows{Table: tableName, Columns: columns, Rows: rows, Total: len(rows), Page: 1, Size: len(rows)})
	}
	return exportData, nil
}

func (d *Database) ImportTables(data []byte, replace bool) error {
	var payload ExportData
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if len(payload.Tables) == 0 {
		return fmt.Errorf("导入文件没有表数据")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, tableData := range payload.Tables {
		if err := d.ensureDataAdminTableName(tableData.Table); err != nil {
			return err
		}
		columns, err := d.TableColumns(tableData.Table)
		if err != nil {
			return err
		}
		columnMap := make(map[string]bool)
		for _, column := range columns {
			columnMap[column.Name] = true
		}

		if replace {
			if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM %s`, quoteIdentifier(tableData.Table))); err != nil {
				return err
			}
		}

		for _, row := range tableData.Rows {
			names := make([]string, 0)
			for name := range row {
				if name != "__rowid__" && columnMap[name] {
					names = append(names, name)
				}
			}
			sort.Strings(names)
			if len(names) == 0 {
				continue
			}

			quotedNames := make([]string, 0, len(names))
			placeholders := make([]string, 0, len(names))
			args := make([]interface{}, 0, len(names))
			for _, name := range names {
				quotedNames = append(quotedNames, quoteIdentifier(name))
				placeholders = append(placeholders, "?")
				args = append(args, normalizeInputValue(row[name]))
			}
			query := fmt.Sprintf(`INSERT OR REPLACE INTO %s (%s) VALUES (%s)`, quoteIdentifier(tableData.Table), strings.Join(quotedNames, ", "), strings.Join(placeholders, ", "))
			if _, err := tx.Exec(query, args...); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (d *Database) editableColumns(table string) ([]ColumnInfo, error) {
	columns, err := d.TableColumns(table)
	if err != nil {
		return nil, err
	}
	editable := make([]ColumnInfo, 0, len(columns))
	for _, column := range columns {
		if column.PrimaryKey || strings.EqualFold(column.Name, "created_at") || strings.EqualFold(column.Name, "updated_at") {
			continue
		}
		editable = append(editable, column)
	}
	return editable, nil
}

func (d *Database) selectAllRows(table string) ([]map[string]interface{}, error) {
	if err := d.ensureTableName(table); err != nil {
		return nil, err
	}
	rows, err := d.db.Query(fmt.Sprintf(`SELECT rowid AS __rowid__, * FROM %s ORDER BY rowid`, quoteIdentifier(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (d *Database) tableRowCount(table string) (int, error) {
	if err := d.ensureTableName(table); err != nil {
		return 0, err
	}
	var count int
	err := d.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteIdentifier(table))).Scan(&count)
	return count, err
}

func (d *Database) ensureTableName(table string) error {
	if !sqlIdentifierPattern.MatchString(table) {
		return fmt.Errorf("表名无效")
	}
	var name string
	err := d.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ? AND name NOT LIKE 'sqlite_%'`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return fmt.Errorf("表不存在: %s", table)
	}
	return err
}

func (d *Database) ensureDataAdminTableName(table string) error {
	if isDataAdminHiddenTable(table) {
		return fmt.Errorf("表不存在: %s", table)
	}
	return d.ensureTableName(table)
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func pluginTableName(pluginID, table string) (string, error) {
	pluginID = strings.TrimSpace(pluginID)
	table = strings.TrimSpace(table)
	if !sqlIdentifierPattern.MatchString(pluginID) {
		return "", fmt.Errorf("插件 ID 无效: %s", pluginID)
	}
	if !sqlIdentifierPattern.MatchString(table) {
		return "", fmt.Errorf("表名无效: %s", table)
	}
	return "plugin_" + pluginID + "_" + table, nil
}

func normalizePluginColumnType(columnType string) string {
	switch strings.ToUpper(strings.TrimSpace(columnType)) {
	case "INTEGER", "INT":
		return "INTEGER"
	case "REAL", "FLOAT", "DOUBLE":
		return "REAL"
	case "BLOB":
		return "BLOB"
	case "DATETIME", "TIMESTAMP":
		return "DATETIME"
	case "BOOLEAN", "BOOL":
		return "INTEGER"
	default:
		return "TEXT"
	}
}

func hasEditableColumn(columns []ColumnInfo, name string) bool {
	for _, column := range columns {
		if strings.EqualFold(column.Name, name) {
			return true
		}
	}
	return false
}

var legacyWhereAndPattern = regexp.MustCompile(`(?i)\s+AND\s+`)
var legacyWhereComparePattern = regexp.MustCompile(`(?i)^([A-Za-z_][A-Za-z0-9_]*)\s*(=|!=|<>|>=|<=|>|<|LIKE)\s*\?$`)
var legacyWhereIsNullPattern = regexp.MustCompile(`(?i)^([A-Za-z_][A-Za-z0-9_]*)\s+IS\s+(NOT\s+)?NULL$`)
var legacyWhereInPattern = regexp.MustCompile(`(?i)^([A-Za-z_][A-Za-z0-9_]*)\s+IN\s*\((\s*\?\s*(,\s*\?\s*)*)\)$`)

func buildPluginWhere(where string, args []interface{}, filters []PluginDBFilter, columnMap map[string]string) (string, []interface{}, error) {
	clauses := make([]string, 0, len(filters)+1)
	queryArgs := make([]interface{}, 0, len(args)+len(filters))

	filterClauses, filterArgs, err := buildPluginFilterClauses(filters, columnMap)
	if err != nil {
		return "", nil, err
	}
	clauses = append(clauses, filterClauses...)
	queryArgs = append(queryArgs, filterArgs...)

	legacyClauses, legacyArgs, err := buildLegacyPluginWhereClauses(where, args, columnMap)
	if err != nil {
		return "", nil, err
	}
	clauses = append(clauses, legacyClauses...)
	queryArgs = append(queryArgs, legacyArgs...)

	if len(clauses) == 0 {
		return "", nil, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), queryArgs, nil
}

func buildPluginFilterClauses(filters []PluginDBFilter, columnMap map[string]string) ([]string, []interface{}, error) {
	clauses := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters))
	for _, filter := range filters {
		field, err := pluginColumnExpression(filter.Field, columnMap)
		if err != nil {
			return nil, nil, err
		}
		rawOp := strings.TrimSpace(filter.Op)
		if rawOp == "" {
			rawOp = strings.TrimSpace(filter.Operator)
		}
		op := "="
		if rawOp != "" {
			op = normalizePluginFilterOp(rawOp)
			if op == "" {
				return nil, nil, fmt.Errorf("查询操作符无效: %s", rawOp)
			}
		}

		switch op {
		case "IS NULL":
			clauses = append(clauses, fmt.Sprintf(`%s IS NULL`, field))
		case "IS NOT NULL":
			clauses = append(clauses, fmt.Sprintf(`%s IS NOT NULL`, field))
		case "IN":
			values := pluginFilterValues(filter)
			if len(values) == 0 {
				return nil, nil, fmt.Errorf("IN 查询值不能为空")
			}
			placeholders := make([]string, 0, len(values))
			for _, value := range values {
				placeholders = append(placeholders, "?")
				args = append(args, normalizeInputValue(value))
			}
			clauses = append(clauses, fmt.Sprintf(`%s IN (%s)`, field, strings.Join(placeholders, ", ")))
		case "=", "<>", ">", ">=", "<", "<=", "LIKE":
			if filter.Value == nil {
				if op == "=" {
					clauses = append(clauses, fmt.Sprintf(`%s IS NULL`, field))
					continue
				}
				if op == "<>" {
					clauses = append(clauses, fmt.Sprintf(`%s IS NOT NULL`, field))
					continue
				}
			}
			clauses = append(clauses, fmt.Sprintf(`%s %s ?`, field, op))
			args = append(args, normalizeInputValue(filter.Value))
		default:
			return nil, nil, fmt.Errorf("查询操作符无效: %s", rawOp)
		}
	}
	return clauses, args, nil
}

func buildLegacyPluginWhereClauses(where string, args []interface{}, columnMap map[string]string) ([]string, []interface{}, error) {
	where = strings.TrimSpace(where)
	if where == "" {
		return nil, nil, nil
	}
	lowerWhere := strings.ToLower(where)
	if strings.Contains(where, ";") || strings.Contains(where, "--") || strings.Contains(lowerWhere, "/*") || strings.Contains(lowerWhere, "*/") {
		return nil, nil, fmt.Errorf("查询条件包含不支持的字符")
	}

	normalizedArgs := normalizePluginArgs(args)
	parts := legacyWhereAndPattern.Split(where, -1)
	clauses := make([]string, 0, len(parts))
	queryArgs := make([]interface{}, 0, len(normalizedArgs))
	argIndex := 0
	for _, part := range parts {
		clause, argCount, err := buildLegacyPluginWhereClause(strings.TrimSpace(part), columnMap)
		if err != nil {
			return nil, nil, err
		}
		if argIndex+argCount > len(normalizedArgs) {
			return nil, nil, fmt.Errorf("查询参数数量不足")
		}
		clauses = append(clauses, clause)
		queryArgs = append(queryArgs, normalizedArgs[argIndex:argIndex+argCount]...)
		argIndex += argCount
	}
	if argIndex != len(normalizedArgs) {
		return nil, nil, fmt.Errorf("查询参数数量不匹配")
	}
	return clauses, queryArgs, nil
}

func buildLegacyPluginWhereClause(clause string, columnMap map[string]string) (string, int, error) {
	if clause == "" {
		return "", 0, fmt.Errorf("查询条件不能为空")
	}
	if matches := legacyWhereComparePattern.FindStringSubmatch(clause); len(matches) == 3 {
		field, err := pluginColumnExpression(matches[1], columnMap)
		if err != nil {
			return "", 0, err
		}
		op := strings.ToUpper(matches[2])
		if op == "!=" {
			op = "<>"
		}
		return fmt.Sprintf(`%s %s ?`, field, op), 1, nil
	}
	if matches := legacyWhereIsNullPattern.FindStringSubmatch(clause); len(matches) == 3 {
		field, err := pluginColumnExpression(matches[1], columnMap)
		if err != nil {
			return "", 0, err
		}
		if strings.TrimSpace(matches[2]) != "" {
			return fmt.Sprintf(`%s IS NOT NULL`, field), 0, nil
		}
		return fmt.Sprintf(`%s IS NULL`, field), 0, nil
	}
	if matches := legacyWhereInPattern.FindStringSubmatch(clause); len(matches) == 4 {
		field, err := pluginColumnExpression(matches[1], columnMap)
		if err != nil {
			return "", 0, err
		}
		placeholderCount := strings.Count(matches[2], "?")
		if placeholderCount == 0 {
			return "", 0, fmt.Errorf("IN 查询值不能为空")
		}
		placeholders := make([]string, 0, placeholderCount)
		for i := 0; i < placeholderCount; i++ {
			placeholders = append(placeholders, "?")
		}
		return fmt.Sprintf(`%s IN (%s)`, field, strings.Join(placeholders, ", ")), placeholderCount, nil
	}
	return "", 0, fmt.Errorf("查询条件仅支持字段与占位符的安全表达式")
}

func buildPluginOrder(order interface{}, orderBy, orderDir string, columnMap map[string]string) (string, error) {
	field := strings.TrimSpace(orderBy)
	direction := strings.TrimSpace(orderDir)
	if field == "" {
		parsedField, parsedDirection, err := parsePluginOrder(order)
		if err != nil {
			return "", err
		}
		field = parsedField
		direction = parsedDirection
	}
	if field == "" {
		return " ORDER BY rowid DESC", nil
	}
	column, err := pluginColumnExpression(field, columnMap)
	if err != nil {
		return "", err
	}
	if direction == "" {
		direction = "ASC"
	}
	direction = strings.ToUpper(strings.TrimSpace(direction))
	if direction != "ASC" && direction != "DESC" {
		return "", fmt.Errorf("排序方向无效")
	}
	return fmt.Sprintf(` ORDER BY %s %s`, column, direction), nil
}

func parsePluginOrder(order interface{}) (string, string, error) {
	switch typed := order.(type) {
	case nil:
		return "", "", nil
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return "", "", nil
		}
		parts := strings.Fields(text)
		if len(parts) == 0 || len(parts) > 2 {
			return "", "", fmt.Errorf("排序字段无效")
		}
		direction := ""
		if len(parts) == 2 {
			direction = parts[1]
		}
		return parts[0], direction, nil
	case map[string]interface{}:
		field := firstPluginString(typed, "field", "column", "order_by", "orderBy")
		direction := firstPluginString(typed, "direction", "dir", "order_dir", "orderDir")
		return field, direction, nil
	default:
		return "", "", fmt.Errorf("排序格式无效")
	}
}

func pluginOrderProvided(order interface{}) bool {
	switch typed := order.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	default:
		return true
	}
}

func pluginColumnMap(columns []ColumnInfo) map[string]string {
	columnMap := map[string]string{"rowid": "rowid"}
	for _, column := range columns {
		columnMap[strings.ToLower(column.Name)] = column.Name
	}
	return columnMap
}

func pluginColumnExpression(field string, columnMap map[string]string) (string, error) {
	field = strings.TrimSpace(field)
	if !sqlIdentifierPattern.MatchString(field) {
		return "", fmt.Errorf("字段名无效: %s", field)
	}
	column, ok := columnMap[strings.ToLower(field)]
	if !ok {
		return "", fmt.Errorf("字段不存在: %s", field)
	}
	if column == "rowid" {
		return "rowid", nil
	}
	return quoteIdentifier(column), nil
}

func normalizePluginFilterOp(op string) string {
	switch strings.ToUpper(strings.TrimSpace(op)) {
	case "":
		return ""
	case "EQ", "=", "==":
		return "="
	case "NE", "!=", "<>":
		return "<>"
	case "GT", ">":
		return ">"
	case "GTE", ">=":
		return ">="
	case "LT", "<":
		return "<"
	case "LTE", "<=":
		return "<="
	case "LIKE":
		return "LIKE"
	case "IN":
		return "IN"
	case "IS NULL", "NULL":
		return "IS NULL"
	case "IS NOT NULL", "NOT NULL":
		return "IS NOT NULL"
	default:
		return ""
	}
}

func pluginFilterValues(filter PluginDBFilter) []interface{} {
	if len(filter.Values) > 0 {
		return filter.Values
	}
	if filter.Value == nil {
		return nil
	}
	if values, ok := filter.Value.([]interface{}); ok {
		return values
	}
	return []interface{}{filter.Value}
}

func firstPluginString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizePluginArgs(args []interface{}) []interface{} {
	result := make([]interface{}, 0, len(args))
	for _, arg := range args {
		result = append(result, normalizeInputValue(arg))
	}
	return result
}

func normalizeInputValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case map[string]interface{}, []interface{}:
		data, _ := json.Marshal(typed)
		return string(data)
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed)
		}
		return typed
	default:
		return typed
	}
}

func normalizeDataViewValues(values map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(values))
	for key, value := range values {
		result[key] = normalizeInputValue(value)
	}

	tableName, _ := result["table_name"].(string)
	tableName = strings.TrimSpace(tableName)
	if tableName != "" {
		result["table_name"] = tableName
		if viewName, ok := result["view_name"].(string); !ok || strings.TrimSpace(viewName) == "" {
			result["view_name"] = tableName
		}
		if groupName, ok := result["group_name"].(string); !ok || strings.TrimSpace(groupName) == "" {
			result["group_name"] = tableGroup(tableName)
		}
	}

	if columns, exists := result["columns"]; exists {
		if columnsText, ok := columns.(string); !ok || strings.TrimSpace(columnsText) == "" {
			result["columns"] = "[]"
		}
	} else {
		result["columns"] = "[]"
	}

	return result
}

func (d *Database) normalizeDataViewsTable() error {
	_, err := d.db.Exec(`
		UPDATE data_views
		SET
			view_name = CASE WHEN TRIM(view_name) = '' THEN table_name ELSE view_name END,
			group_name = CASE WHEN TRIM(group_name) = '' THEN '业务数据' ELSE group_name END,
			columns = CASE WHEN TRIM(columns) = '' THEN '[]' ELSE columns END,
			updated_at = CURRENT_TIMESTAMP
	`)
	return err
}

func normalizeDBValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	default:
		return typed
	}
}

func scanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columnNames))
		valuePtrs := make([]interface{}, len(columnNames))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		item := make(map[string]interface{}, len(columnNames))
		for i, name := range columnNames {
			item[name] = normalizeDBValue(values[i])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func tableGroup(name string) string {
	switch {
	case strings.Contains(name, "setting") || strings.Contains(name, "config"):
		return "系统配置"
	case strings.Contains(name, "adapter"):
		return "机器人配置"
	case strings.Contains(name, "plugin"):
		return "插件数据"
	default:
		return "业务数据"
	}
}
