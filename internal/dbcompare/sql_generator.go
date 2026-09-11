package dbcompare

import (
	"fmt"
	"strings"
)

const (
	sqlDatabaseTypeMySQL    = "mysql"
	sqlDatabaseTypePostgres = "postgres"
	sqlDatabaseTypeSQLite   = "sqlite"
)

func GenerateSQL(diff *SchemaDiff, dbType string) []string {
	var sqls []string
	// 新增外键约束统一放到脚本最后，避免被引用的表尚未创建导致执行失败。
	var foreignKeySQLs []string
	for _, tableDiff := range diff.TableDiffs {
		switch tableDiff.Type {
		case DiffTypeCreate:
			if tableDiff.SourceTable != nil {
				sqls = append(sqls, generateCreateTableSQL(tableDiff.SourceTable, dbType))
				for indexIndex := range tableDiff.SourceTable.Indexes {
					if indexSQL := generateAddIndexSQL(tableDiff.TableName, &tableDiff.SourceTable.Indexes[indexIndex], dbType); indexSQL != "" {
						sqls = append(sqls, indexSQL)
					}
				}
				for fkIndex := range tableDiff.SourceTable.ForeignKeys {
					if fkSQL := generateAddForeignKeySQL(tableDiff.TableName, &tableDiff.SourceTable.ForeignKeys[fkIndex], dbType); fkSQL != "" {
						foreignKeySQLs = append(foreignKeySQLs, fkSQL)
					}
				}
			}
		case DiffTypeAlter:
			structuralSQLs, addForeignKeySQLs := generateAlterTableSQL(&tableDiff, dbType)
			sqls = append(sqls, structuralSQLs...)
			foreignKeySQLs = append(foreignKeySQLs, addForeignKeySQLs...)
		case DiffTypeDrop:
			sqls = append(sqls, generateDropTableSQL(tableDiff.TableName, dbType))
		}
	}
	return append(sqls, foreignKeySQLs...)
}

func generateCreateTableSQL(table *Table, dbType string) string {
	var definitions []string
	var primaryKeyColumns []string
	for _, col := range table.Columns {
		definitions = append(definitions, generateColumnDefinition(&col, dbType))
		if col.PrimaryKey {
			primaryKeyColumns = append(primaryKeyColumns, quoteIdentifier(col.Name))
		}
	}

	if len(primaryKeyColumns) > 0 {
		definitions = append(definitions, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(primaryKeyColumns, ", ")))
	}

	tableOptions := ""
	if dbType == sqlDatabaseTypeMySQL && table.Comment != "" {
		tableOptions = fmt.Sprintf(" COMMENT=%s", quoteStringLiteral(table.Comment))
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)%s;", quoteIdentifier(table.Name), strings.Join(definitions, ",\n  "), tableOptions)
}

// generateAlterTableSQL 返回结构变更 SQL 和需要延后执行的新增外键 SQL。
// 语句顺序保证先删除约束/索引，再改字段，最后建索引、加外键，避免依赖冲突。
func generateAlterTableSQL(tableDiff *TableDiff, dbType string) ([]string, []string) {
	var sqls []string
	var foreignKeySQLs []string

	// 1. 先删除发生变更或多余的外键约束（删除列/索引前必须先解除外键）
	for _, fkDiff := range tableDiff.ForeignKeyDiffs {
		switch fkDiff.Type {
		case DiffTypeAlter, DiffTypeDrop:
			if fkDiff.TargetForeignKey != nil {
				if sql := generateDropForeignKeySQL(tableDiff.TableName, fkDiff.TargetForeignKey.Name, dbType); sql != "" {
					sqls = append(sqls, sql)
				}
			}
		}
	}

	// 2. 删除发生变更或多余的索引
	for _, indexDiff := range tableDiff.IndexDiffs {
		switch indexDiff.Type {
		case DiffTypeAlter, DiffTypeDrop:
			if indexDiff.TargetIndex != nil {
				if sql := generateDropIndexSQL(tableDiff.TableName, indexDiff.TargetIndex.Name, dbType); sql != "" {
					sqls = append(sqls, sql)
				}
			}
		}
	}

	// 3. 表备注与字段变更
	if tableDiff.TableCommentChanged && tableDiff.SourceTable != nil {
		sqls = append(sqls, generateAlterTableCommentSQL(tableDiff.SourceTable, dbType))
	}
	for _, colDiff := range tableDiff.ColumnDiffs {
		switch colDiff.Type {
		case DiffTypeCreate:
			if colDiff.SourceColumn != nil {
				sqls = append(sqls, generateAddColumnSQL(tableDiff.TableName, colDiff.SourceColumn, dbType))
			}
		case DiffTypeAlter:
			if colDiff.SourceColumn != nil {
				sqls = append(sqls, generateModifyColumnSQL(tableDiff.TableName, colDiff.SourceColumn, dbType))
			}
		case DiffTypeDrop:
			sqls = append(sqls, generateDropColumnSQL(tableDiff.TableName, colDiff.ColumnName, dbType))
		}
	}

	// 4. 新建索引（含变更索引的重建）
	for _, indexDiff := range tableDiff.IndexDiffs {
		switch indexDiff.Type {
		case DiffTypeCreate, DiffTypeAlter:
			if indexDiff.SourceIndex != nil {
				if sql := generateAddIndexSQL(tableDiff.TableName, indexDiff.SourceIndex, dbType); sql != "" {
					sqls = append(sqls, sql)
				}
			}
		}
	}

	// 5. 新建外键约束（含变更外键的重建），由调用方延后到所有表结构变更完成后执行
	for _, fkDiff := range tableDiff.ForeignKeyDiffs {
		switch fkDiff.Type {
		case DiffTypeCreate, DiffTypeAlter:
			if fkDiff.SourceForeignKey != nil {
				if sql := generateAddForeignKeySQL(tableDiff.TableName, fkDiff.SourceForeignKey, dbType); sql != "" {
					foreignKeySQLs = append(foreignKeySQLs, sql)
				}
			}
		}
	}

	return sqls, foreignKeySQLs
}

func generateAddColumnSQL(tableName string, column *Column, dbType string) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", quoteIdentifier(tableName), generateColumnDefinition(column, dbType))
}

func generateModifyColumnSQL(tableName string, column *Column, dbType string) string {
	var sql string
	colDef := generateColumnDefinition(column, dbType)

	switch dbType {
	case sqlDatabaseTypeMySQL:
		sql = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;", quoteIdentifier(tableName), colDef)
	case sqlDatabaseTypePostgres:
		sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", tableName, column.Name, column.Type)
		if !column.Nullable {
			sql += fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", tableName, column.Name)
		} else {
			sql += fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", tableName, column.Name)
		}
	case sqlDatabaseTypeSQLite:
		return "-- SQLite does not support MODIFY COLUMN"
	default:
		return fmt.Sprintf("-- Unsupported database type: %s", dbType)
	}

	return sql
}

func generateDropColumnSQL(tableName, columnName string, dbType string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quoteIdentifier(tableName), quoteIdentifier(columnName))
}

func generateDropTableSQL(tableName string, dbType string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s;", quoteIdentifier(tableName))
}

func generateColumnDefinition(column *Column, dbType string) string {
	definition := fmt.Sprintf("%s %s", quoteIdentifier(column.Name), column.Type)

	if !column.Nullable || column.PrimaryKey {
		definition += " NOT NULL"
	}
	if column.AutoIncrement {
		switch dbType {
		case sqlDatabaseTypeMySQL:
			definition += " AUTO_INCREMENT"
		case sqlDatabaseTypePostgres:
			definition += " GENERATED ALWAYS AS IDENTITY"
		case sqlDatabaseTypeSQLite:
		}
	}
	if column.DefaultValue != "" {
		definition += fmt.Sprintf(" DEFAULT %s", quoteDefaultValue(column.DefaultValue, column.Type))
	}
	if dbType == sqlDatabaseTypeMySQL && column.Comment != "" {
		definition += fmt.Sprintf(" COMMENT %s", quoteStringLiteral(column.Comment))
	}

	return definition
}

func quoteIdentifier(identifier string) string {
	return fmt.Sprintf("`%s`", strings.ReplaceAll(identifier, "`", "``"))
}

func quoteStringLiteral(value string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "''"))
}

// quoteDefaultValue wraps string-type default values in single quotes if not already quoted.
// Numeric values, SQL functions/expressions, and already-quoted values are returned as-is.
func quoteDefaultValue(defaultValue, colType string) string {
	val := strings.TrimSpace(defaultValue)
	if val == "" || val == "NULL" || val == "null" {
		return val
	}

	// Already quoted (PostgreSQL returns 'value'::text style)
	if strings.HasPrefix(val, "'") {
		return val
	}

	// SQL functions / expressions that should not be quoted
	upperVal := strings.ToUpper(val)
	sqlFunctions := []string{
		"CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME",
		"NOW()", "CURRENT_TIMESTAMP()", "UTC_TIMESTAMP", "UTC_TIMESTAMP()",
		"UNIX_TIMESTAMP", "UNIX_TIMESTAMP()", "UUID()", "RAND()",
		"CURRENT_USER", "CURRENT_USER()", "SESSION_USER", "USER()",
		"TRUE", "FALSE", "true", "false",
	}
	for _, fn := range sqlFunctions {
		if upperVal == fn || val == fn {
			return val
		}
	}

	// PostgreSQL expressions like nextval('seq_name'::regclass)
	if strings.Contains(val, "(") && strings.Contains(val, ")") {
		return val
	}

	// Numeric values (int, float, decimal, including negative)
	if isNumeric(val) {
		return val
	}

	// String types need quoting
	colTypeLower := strings.ToLower(colType)
	stringTypes := []string{
		"char", "varchar", "text", "enum", "set", "json",
		"tinytext", "mediumtext", "longtext", "blob",
		"tinyblob", "mediumblob", "longblob", "binary", "varbinary",
	}
	for _, st := range stringTypes {
		if strings.Contains(colTypeLower, st) {
			return quoteStringLiteral(val)
		}
	}

	// Default: if not numeric and not a known function, treat as string
	return quoteStringLiteral(val)
}

// isNumeric checks if a string represents a numeric value (int, float, decimal).
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	// Allow leading negative sign
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	hasDigit := false
	hasDot := false
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case c == '.' && !hasDot:
			hasDot = true
		default:
			return false
		}
	}
	return hasDigit
}

func generateAlterTableCommentSQL(table *Table, dbType string) string {
	switch dbType {
	case sqlDatabaseTypeMySQL:
		return fmt.Sprintf("ALTER TABLE %s COMMENT = %s;", quoteIdentifier(table.Name), quoteStringLiteral(table.Comment))
	default:
		return fmt.Sprintf("-- Unsupported table comment change for database type: %s", dbType)
	}
}

func generateAddIndexSQL(tableName string, index *Index, dbType string) string {
	if strings.EqualFold(index.Name, primaryKeyIndexName) {
		return ""
	}
	columns := index.Columns
	if len(columns) == 0 && index.Column != "" {
		columns = []string{index.Column}
	}
	quotedColumns := quoteIdentifierList(columns, dbType)
	if len(quotedColumns) == 0 {
		return ""
	}

	uniqueKeyword := ""
	if index.Unique {
		uniqueKeyword = "UNIQUE "
	}

	switch dbType {
	case sqlDatabaseTypeMySQL:
		return fmt.Sprintf("ALTER TABLE %s ADD %sINDEX %s (%s);",
			quoteIdentifier(tableName), uniqueKeyword, quoteIdentifier(index.Name), strings.Join(quotedColumns, ", "))
	case sqlDatabaseTypePostgres:
		return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s);",
			uniqueKeyword, quoteIdentifierByDB(index.Name, dbType), quoteIdentifierByDB(tableName, dbType), strings.Join(quotedColumns, ", "))
	case sqlDatabaseTypeSQLite:
		return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s);",
			uniqueKeyword, quoteIdentifier(index.Name), quoteIdentifier(tableName), strings.Join(quotedColumns, ", "))
	default:
		return fmt.Sprintf("-- Unsupported database type: %s", dbType)
	}
}

func generateDropIndexSQL(tableName, indexName string, dbType string) string {
	if strings.EqualFold(indexName, primaryKeyIndexName) {
		return ""
	}
	switch dbType {
	case sqlDatabaseTypeMySQL:
		return fmt.Sprintf("ALTER TABLE %s DROP INDEX %s;", quoteIdentifier(tableName), quoteIdentifier(indexName))
	case sqlDatabaseTypePostgres:
		return fmt.Sprintf("DROP INDEX IF EXISTS %s;", quoteIdentifierByDB(indexName, dbType))
	case sqlDatabaseTypeSQLite:
		return fmt.Sprintf("DROP INDEX IF EXISTS %s;", quoteIdentifier(indexName))
	default:
		return fmt.Sprintf("-- Unsupported database type: %s", dbType)
	}
}

func generateAddForeignKeySQL(tableName string, foreignKey *ForeignKey, dbType string) string {
	fromColumns := foreignKeyFromColumns(foreignKey)
	toColumns := foreignKeyToColumns(foreignKey)
	if len(fromColumns) == 0 || foreignKey.ToTable == "" || len(toColumns) == 0 {
		return ""
	}

	constraintName := foreignKey.Name
	if constraintName == "" {
		constraintName = fmt.Sprintf("fk_%s_%s", tableName, fromColumns[0])
	}

	switch dbType {
	case sqlDatabaseTypeMySQL, sqlDatabaseTypePostgres:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)%s%s;",
			quoteIdentifierByDB(tableName, dbType),
			quoteIdentifierByDB(constraintName, dbType),
			strings.Join(quoteIdentifierList(fromColumns, dbType), ", "),
			quoteIdentifierByDB(foreignKey.ToTable, dbType),
			strings.Join(quoteIdentifierList(toColumns, dbType), ", "),
			generateForeignKeyRuleClause("ON DELETE", foreignKey.OnDelete),
			generateForeignKeyRuleClause("ON UPDATE", foreignKey.OnUpdate),
		)
	case sqlDatabaseTypeSQLite:
		return fmt.Sprintf("-- SQLite does not support adding foreign key constraint %s via ALTER TABLE", constraintName)
	default:
		return fmt.Sprintf("-- Unsupported database type: %s", dbType)
	}
}

func generateDropForeignKeySQL(tableName, foreignKeyName string, dbType string) string {
	if foreignKeyName == "" {
		return ""
	}
	switch dbType {
	case sqlDatabaseTypeMySQL:
		return fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s;", quoteIdentifier(tableName), quoteIdentifier(foreignKeyName))
	case sqlDatabaseTypePostgres:
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", quoteIdentifierByDB(tableName, dbType), quoteIdentifierByDB(foreignKeyName, dbType))
	case sqlDatabaseTypeSQLite:
		return fmt.Sprintf("-- SQLite does not support dropping foreign key constraint %s via ALTER TABLE", foreignKeyName)
	default:
		return fmt.Sprintf("-- Unsupported database type: %s", dbType)
	}
}

func generateForeignKeyRuleClause(clause, rule string) string {
	normalized := strings.ToUpper(strings.TrimSpace(rule))
	switch normalized {
	case "CASCADE", "SET NULL", "SET DEFAULT", "RESTRICT", "NO ACTION":
		return fmt.Sprintf(" %s %s", clause, normalized)
	default:
		return ""
	}
}

func quoteIdentifierByDB(identifier, dbType string) string {
	if dbType == sqlDatabaseTypePostgres {
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(identifier, "\"", "\"\""))
	}
	return quoteIdentifier(identifier)
}

func quoteIdentifierList(identifiers []string, dbType string) []string {
	quoted := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		identifier = strings.TrimSpace(identifier)
		if identifier == "" {
			continue
		}
		quoted = append(quoted, quoteIdentifierByDB(identifier, dbType))
	}
	return quoted
}
