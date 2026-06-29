package datasync

import (
	"fmt"
	"strings"

	"dbsync/internal/dbcompare"
)

type SyncResult struct {
	Table      string `json:"table"`
	Inserted   int    `json:"inserted"`
	Updated    int    `json:"updated"`
	Deleted    int    `json:"deleted"`
	Total      int    `json:"total"`
	Error      string `json:"error,omitempty"`
}

func SyncTableData(source, target *dbcompare.DBConnection, tableName string, deleteMissing bool) (*SyncResult, error) {
	result := &SyncResult{Table: tableName}

	sourceSchema, err := source.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read source schema: %w", err)
	}

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

	sourceData, err := readTableData(source, sourceTable)
	if err != nil {
		return nil, fmt.Errorf("failed to read source data: %w", err)
	}

	targetData, err := readTableData(target, targetTable)
	if err != nil {
		return nil, fmt.Errorf("failed to read target data: %w", err)
	}

	result.Total = len(sourceData)

	sourcePKMap := buildPKMap(sourceData, pkColumns)
	targetPKMap := buildPKMap(targetData, pkColumns)

	for pk, sourceRow := range sourcePKMap {
		if targetRow, ok := targetPKMap[pk]; ok {
			if !rowsEqual(sourceRow, targetRow) {
				if err := updateRow(target, targetTable, sourceRow, pkColumns); err != nil {
					return nil, fmt.Errorf("failed to update row: %w", err)
				}
				result.Updated++
			}
			delete(targetPKMap, pk)
		} else {
			if err := insertRow(target, targetTable, sourceRow); err != nil {
				return nil, fmt.Errorf("failed to insert row: %w", err)
			}
			result.Inserted++
		}
	}

	if deleteMissing {
		for pk := range targetPKMap {
			if err := deleteRow(target, targetTable, pk, pkColumns); err != nil {
				return nil, fmt.Errorf("failed to delete row: %w", err)
			}
			result.Deleted++
		}
	}

	return result, nil
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

func insertRow(db *dbcompare.DBConnection, table *dbcompare.Table, row map[string]interface{}) error {
	var cols []string
	var placeholders []string
	var values []interface{}

	for _, col := range table.Columns {
		if col.AutoIncrement {
			continue
		}
		cols = append(cols, fmt.Sprintf("`%s`", col.Name))
		placeholders = append(placeholders, "?")
		values = append(values, row[col.Name])
	}

	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", table.Name, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	_, err := db.DB.Exec(sql, values...)
	if err != nil {
		return fmt.Errorf("failed to insert: %w", err)
	}

	return nil
}

func updateRow(db *dbcompare.DBConnection, table *dbcompare.Table, row map[string]interface{}, pkColumns []string) error {
	var setClauses []string
	var values []interface{}

	for _, col := range table.Columns {
		if col.PrimaryKey || col.AutoIncrement {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("`%s` = ?", col.Name))
		values = append(values, row[col.Name])
	}

	var whereClauses []string
	for _, pkCol := range pkColumns {
		whereClauses = append(whereClauses, fmt.Sprintf("`%s` = ?", pkCol))
		values = append(values, row[pkCol])
	}

	sql := fmt.Sprintf("UPDATE `%s` SET %s WHERE %s", table.Name, strings.Join(setClauses, ", "), strings.Join(whereClauses, " AND "))
	_, err := db.DB.Exec(sql, values...)
	if err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	return nil
}

func deleteRow(db *dbcompare.DBConnection, table *dbcompare.Table, pk string, pkColumns []string) error {
	pkParts := strings.Split(pk, "|")
	var whereClauses []string
	var values []interface{}

	for i, pkCol := range pkColumns {
		whereClauses = append(whereClauses, fmt.Sprintf("`%s` = ?", pkCol))
		values = append(values, pkParts[i])
	}

	sql := fmt.Sprintf("DELETE FROM `%s` WHERE %s", table.Name, strings.Join(whereClauses, " AND "))
	_, err := db.DB.Exec(sql, values...)
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

func SyncAllTables(source, target *dbcompare.DBConnection, deleteMissing bool) ([]*SyncResult, error) {
	sourceSchema, err := source.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read source schema: %w", err)
	}

	var results []*SyncResult
	for _, table := range sourceSchema.Tables {
		result, err := SyncTableData(source, target, table.Name, deleteMissing)
		if err != nil {
			results = append(results, &SyncResult{
				Table: table.Name,
				Error: err.Error(),
			})
		} else {
			results = append(results, result)
		}
	}

	return results, nil
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
