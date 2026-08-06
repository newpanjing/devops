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
	for _, tableDiff := range diff.TableDiffs {
		switch tableDiff.Type {
		case DiffTypeCreate:
			if tableDiff.SourceTable != nil {
				sqls = append(sqls, generateCreateTableSQL(tableDiff.SourceTable, dbType))
			}
		case DiffTypeAlter:
			sqls = append(sqls, generateAlterTableSQL(&tableDiff, dbType)...)
		case DiffTypeDrop:
			sqls = append(sqls, generateDropTableSQL(tableDiff.TableName, dbType))
		}
	}
	return sqls
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

func generateAlterTableSQL(tableDiff *TableDiff, dbType string) []string {
	var sqls []string
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
	return sqls
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
		definition += fmt.Sprintf(" DEFAULT %s", column.DefaultValue)
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

func generateAlterTableCommentSQL(table *Table, dbType string) string {
	switch dbType {
	case sqlDatabaseTypeMySQL:
		return fmt.Sprintf("ALTER TABLE %s COMMENT = %s;", quoteIdentifier(table.Name), quoteStringLiteral(table.Comment))
	default:
		return fmt.Sprintf("-- Unsupported table comment change for database type: %s", dbType)
	}
}
