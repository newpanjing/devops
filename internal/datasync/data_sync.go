package datasync

import (
	"fmt"
	"strings"

	"dbsync/internal/dbcompare"
)

type SyncResult struct {
	Table    string   `json:"table"`
	Inserted int      `json:"inserted"`
	Updated  int      `json:"updated"`
	Deleted  int      `json:"deleted"`
	Total    int      `json:"total"`
	Error    string   `json:"error,omitempty"`
	SQL      []string `json:"sql"`
}

type DataSyncProgress struct {
	TableName string
	Message   string
}

type DataSyncProgressFunc func(progress DataSyncProgress)

func PreviewTableData(source, target *dbcompare.DBConnection, tableName string, deleteMissing bool) (*SyncResult, error) {
	return PreviewTableDataWithProgress(source, target, tableName, deleteMissing, nil)
}

func PreviewTableDataWithProgress(source, target *dbcompare.DBConnection, tableName string, deleteMissing bool, progressFunc DataSyncProgressFunc) (*SyncResult, error) {
	result := &SyncResult{Table: tableName}

	notifyDataSyncProgress(progressFunc, DataSyncProgress{TableName: tableName, Message: "正在读取源表结构"})
	sourceSchema, err := source.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read source schema: %w", err)
	}

	notifyDataSyncProgress(progressFunc, DataSyncProgress{TableName: tableName, Message: "正在读取目标表结构"})
	targetSchema, err := target.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read target schema: %w", err)
	}

	var sourceTable, targetTable *dbcompare.Table
	for i := range sourceSchema.Tables {
		if sourceSchema.Tables[i].Name == tableName {
			sourceTable = &sourceSchema.Tables[i]
			break
		}
	}
	if sourceTable == nil {
		return nil, fmt.Errorf("table %s not found in source", tableName)
	}

	for i := range targetSchema.Tables {
		if targetSchema.Tables[i].Name == tableName {
			targetTable = &targetSchema.Tables[i]
			break
		}
	}
	if targetTable == nil {
		return nil, fmt.Errorf("table %s not found in target", tableName)
	}

	pkColumns := getPrimaryKeyColumns(sourceTable)
	if len(pkColumns) == 0 {
		return nil, fmt.Errorf("table %s has no primary key", tableName)
	}

	notifyDataSyncProgress(progressFunc, DataSyncProgress{TableName: tableName, Message: "正在读取源表数据"})
	sourceData, err := readTableData(source, sourceTable)
	if err != nil {
		return nil, fmt.Errorf("failed to read source data: %w", err)
	}

	notifyDataSyncProgress(progressFunc, DataSyncProgress{TableName: tableName, Message: "正在读取目标表数据"})
	targetData, err := readTableData(target, targetTable)
	if err != nil {
		return nil, fmt.Errorf("failed to read target data: %w", err)
	}

	result.Total = len(sourceData)
	notifyDataSyncProgress(progressFunc, DataSyncProgress{
		TableName: tableName,
		Message:   fmt.Sprintf("源表 %d 行，目标表 %d 行，正在按主键比对", len(sourceData), len(targetData)),
	})

	sourcePKMap := buildPKMap(sourceData, pkColumns)
	targetPKMap := buildPKMap(targetData, pkColumns)

	for pk, sourceRow := range sourcePKMap {
		if targetRow, ok := targetPKMap[pk]; ok {
			if !rowsEqual(sourceRow, targetRow) {
				sql := buildUpdateSQL(targetTable, sourceRow, pkColumns)
				result.SQL = append(result.SQL, sql)
				result.Updated++
				notifyDataSyncProgress(progressFunc, DataSyncProgress{TableName: tableName, Message: fmt.Sprintf("发现修改记录，主键: %s", pk)})
			}
			delete(targetPKMap, pk)
		} else {
			sql := buildInsertSQL(targetTable, sourceRow)
			result.SQL = append(result.SQL, sql)
			result.Inserted++
			notifyDataSyncProgress(progressFunc, DataSyncProgress{TableName: tableName, Message: fmt.Sprintf("发现新增记录，主键: %s", pk)})
		}
	}

	if deleteMissing {
		for pk := range targetPKMap {
			sql := buildDeleteSQL(targetTable, pk, pkColumns)
			result.SQL = append(result.SQL, sql)
			result.Deleted++
			notifyDataSyncProgress(progressFunc, DataSyncProgress{TableName: tableName, Message: fmt.Sprintf("发现删除记录，主键: %s", pk)})
		}
	}

	notifyDataSyncProgress(progressFunc, DataSyncProgress{
		TableName: tableName,
		Message:   fmt.Sprintf("表比对完成，新增 %d，修改 %d，删除 %d", result.Inserted, result.Updated, result.Deleted),
	})

	return result, nil
}

func notifyDataSyncProgress(progressFunc DataSyncProgressFunc, progress DataSyncProgress) {
	if progressFunc != nil {
		progressFunc(progress)
	}
}

func getPrimaryKeyColumns(table *dbcompare.Table) []string {
	var cols []string
	for _, col := range table.Columns {
		if col.PrimaryKey {
			cols = append(cols, col.Name)
		}
	}
	return cols
}

func readTableData(db *dbcompare.DBConnection, table *dbcompare.Table) ([]map[string]interface{}, error) {
	cols := make([]string, 0, len(table.Columns))
	for _, col := range table.Columns {
		cols = append(cols, fmt.Sprintf("`%s`", col.Name))
	}

	sql := fmt.Sprintf("SELECT %s FROM `%s`", strings.Join(cols, ", "), table.Name)
	rows, err := db.DB.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to query table: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		result = append(result, row)
	}

	return result, nil
}

func buildPKMap(data []map[string]interface{}, pkColumns []string) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	for _, row := range data {
		var pkParts []string
		for _, pkCol := range pkColumns {
			pkParts = append(pkParts, fmt.Sprintf("%v", row[pkCol]))
		}
		pk := strings.Join(pkParts, "|")
		result[pk] = row
	}
	return result
}

func rowsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !valuesEqual(v, bv) {
			return false
		}
	}
	return true
}

func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	switch av := a.(type) {
	case []byte:
		bv, ok := b.([]byte)
		if !ok {
			return false
		}
		return string(av) == string(bv)
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

func CountTableRows(db *dbcompare.DBConnection, tableName string) (int, error) {
	var count int
	err := db.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}
	return count, nil
}

func GetTableRowSample(db *dbcompare.DBConnection, tableName string, limit int) ([]map[string]interface{}, error) {
	schema, err := db.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	var table *dbcompare.Table
	for i := range schema.Tables {
		if schema.Tables[i].Name == tableName {
			table = &schema.Tables[i]
			break
		}
	}
	if table == nil {
		return nil, fmt.Errorf("table %s not found", tableName)
	}

	cols := make([]string, 0, len(table.Columns))
	for _, col := range table.Columns {
		cols = append(cols, fmt.Sprintf("`%s`", col.Name))
	}

	sql := fmt.Sprintf("SELECT %s FROM `%s` LIMIT %d", strings.Join(cols, ", "), tableName, limit)
	rows, err := db.DB.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to query table: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			var val interface{} = values[i]
			if bytes, ok := val.([]byte); ok {
				val = string(bytes)
			}
			if val == nil {
				val = "NULL"
			}
			row[col] = val
		}
		result = append(result, row)
	}

	return result, nil
}

func TestConnection(dbType, host string, port int, database, username, password string) error {
	db := &dbcompare.DBConnection{
		DBType:   dbType,
		Host:     host,
		Port:     port,
		Database: database,
		Username: username,
		Password: password,
	}
	return db.Connect()
}

func GetTableList(db *dbcompare.DBConnection) ([]string, error) {
	schema, err := db.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	var tables []string
	for _, table := range schema.Tables {
		tables = append(tables, table.Name)
	}
	return tables, nil
}

func buildInsertSQL(table *dbcompare.Table, row map[string]interface{}) string {
	var cols []string
	var values []string

	for _, col := range table.Columns {
		if col.AutoIncrement {
			continue
		}
		cols = append(cols, fmt.Sprintf("`%s`", col.Name))
		values = append(values, formatSQLValue(row[col.Name]))
	}

	return fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s);", table.Name, strings.Join(cols, ", "), strings.Join(values, ", "))
}

func buildUpdateSQL(table *dbcompare.Table, row map[string]interface{}, pkColumns []string) string {
	var setClauses []string
	var whereClauses []string

	for _, col := range table.Columns {
		if col.PrimaryKey || col.AutoIncrement {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("`%s` = %s", col.Name, formatSQLValue(row[col.Name])))
	}

	for _, pkCol := range pkColumns {
		whereClauses = append(whereClauses, fmt.Sprintf("`%s` = %s", pkCol, formatSQLValue(row[pkCol])))
	}

	return fmt.Sprintf("UPDATE `%s` SET %s WHERE %s;", table.Name, strings.Join(setClauses, ", "), strings.Join(whereClauses, " AND "))
}

func buildDeleteSQL(table *dbcompare.Table, pk string, pkColumns []string) string {
	pkParts := strings.Split(pk, "|")
	var whereClauses []string

	for i, pkCol := range pkColumns {
		whereClauses = append(whereClauses, fmt.Sprintf("`%s` = %s", pkCol, formatSQLValue(pkParts[i])))
	}

	return fmt.Sprintf("DELETE FROM `%s` WHERE %s;", table.Name, strings.Join(whereClauses, " AND "))
}

func formatSQLValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case []byte:
		return fmt.Sprintf("'%s'", escapeSQL(string(val)))
	case string:
		return fmt.Sprintf("'%s'", escapeSQL(val))
	case int, int64, int32, int16, int8:
		return fmt.Sprintf("%d", val)
	case uint, uint64, uint32, uint16, uint8:
		return fmt.Sprintf("%d", val)
	case float64, float32:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprintf("'%s'", escapeSQL(fmt.Sprintf("%v", val)))
	}
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "'", "\\'")
}
