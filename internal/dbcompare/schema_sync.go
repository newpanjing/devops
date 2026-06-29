package dbcompare

import (
	"fmt"
	"strings"
)

func (dc *DBConnection) SyncSchema(diff *SchemaDiff) error {
	for _, tableDiff := range diff.TableDiffs {
		if tableDiff.Type == DiffTypeCreate {
			if err := dc.createTable(tableDiff.SourceTable); err != nil {
				return fmt.Errorf("failed to create table %s: %w", tableDiff.TableName, err)
			}
		} else if tableDiff.Type == DiffTypeAlter {
			if err := dc.alterTable(&tableDiff); err != nil {
				return fmt.Errorf("failed to alter table %s: %w", tableDiff.TableName, err)
			}
		} else if tableDiff.Type == DiffTypeDrop {
			if err := dc.dropTable(tableDiff.TableName); err != nil {
				return fmt.Errorf("failed to drop table %s: %w", tableDiff.TableName, err)
			}
		}
	}
	return nil
}

func (dc *DBConnection) createTable(table *Table) error {
	var cols []string
	for _, col := range table.Columns {
		colDef := fmt.Sprintf("`%s` %s", col.Name, col.Type)
		if col.PrimaryKey {
			colDef += " PRIMARY KEY"
		}
		if col.AutoIncrement {
			switch dc.DBType {
			case "mysql":
				colDef += " AUTO_INCREMENT"
			case "postgres":
				colDef += " GENERATED ALWAYS AS IDENTITY"
			case "sqlite":
			}
		}
		if !col.Nullable {
			colDef += " NOT NULL"
		}
		if col.DefaultValue != "" {
			colDef += fmt.Sprintf(" DEFAULT %s", col.DefaultValue)
		}
		cols = append(cols, colDef)
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (%s)", table.Name, strings.Join(cols, ", "))
	_, err := dc.DB.Exec(sql)
	if err != nil {
		return fmt.Errorf("failed to execute CREATE TABLE: %w", err)
	}

	return nil
}

func (dc *DBConnection) alterTable(tableDiff *TableDiff) error {
	for _, colDiff := range tableDiff.ColumnDiffs {
		switch colDiff.Type {
		case DiffTypeCreate:
			if err := dc.addColumn(tableDiff.TableName, colDiff.SourceColumn); err != nil {
				return fmt.Errorf("failed to add column %s: %w", colDiff.ColumnName, err)
			}
		case DiffTypeAlter:
			if err := dc.modifyColumn(tableDiff.TableName, colDiff.SourceColumn); err != nil {
				return fmt.Errorf("failed to modify column %s: %w", colDiff.ColumnName, err)
			}
		case DiffTypeDrop:
			if err := dc.dropColumn(tableDiff.TableName, colDiff.ColumnName); err != nil {
				return fmt.Errorf("failed to drop column %s: %w", colDiff.ColumnName, err)
			}
		}
	}
	return nil
}

func (dc *DBConnection) addColumn(tableName string, column *Column) error {
	colDef := fmt.Sprintf("`%s` %s", column.Name, column.Type)
	if column.PrimaryKey {
		colDef += " PRIMARY KEY"
	}
	if column.AutoIncrement {
		switch dc.DBType {
		case "mysql":
			colDef += " AUTO_INCREMENT"
		case "postgres":
			colDef += " GENERATED ALWAYS AS IDENTITY"
		}
	}
	if !column.Nullable {
		colDef += " NOT NULL"
	}
	if column.DefaultValue != "" {
		colDef += fmt.Sprintf(" DEFAULT %s", column.DefaultValue)
	}

	sql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", tableName, colDef)
	_, err := dc.DB.Exec(sql)
	if err != nil {
		return fmt.Errorf("failed to execute ADD COLUMN: %w", err)
	}

	return nil
}

func (dc *DBConnection) modifyColumn(tableName string, column *Column) error {
	var sql string
	colDef := fmt.Sprintf("`%s` %s", column.Name, column.Type)
	if !column.Nullable {
		colDef += " NOT NULL"
	} else {
		colDef += " NULL"
	}
	if column.DefaultValue != "" {
		colDef += fmt.Sprintf(" DEFAULT %s", column.DefaultValue)
	}

	switch dc.DBType {
	case "mysql":
		sql = fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN %s", tableName, colDef)
	case "postgres":
		sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s", tableName, column.Name, column.Type)
		if !column.Nullable {
			sql += fmt.Sprintf(", ALTER COLUMN %s SET NOT NULL", column.Name)
		} else {
			sql += fmt.Sprintf(", ALTER COLUMN %s DROP NOT NULL", column.Name)
		}
	case "sqlite":
		return fmt.Errorf("SQLite does not support MODIFY COLUMN")
	default:
		return fmt.Errorf("unsupported database type: %s", dc.DBType)
	}

	_, err := dc.DB.Exec(sql)
	if err != nil {
		return fmt.Errorf("failed to execute MODIFY COLUMN: %w", err)
	}

	return nil
}

func (dc *DBConnection) dropColumn(tableName, columnName string) error {
	sql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", tableName, columnName)
	_, err := dc.DB.Exec(sql)
	if err != nil {
		return fmt.Errorf("failed to execute DROP COLUMN: %w", err)
	}
	return nil
}

func (dc *DBConnection) dropTable(tableName string) error {
	sql := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)
	_, err := dc.DB.Exec(sql)
	if err != nil {
		return fmt.Errorf("failed to execute DROP TABLE: %w", err)
	}
	return nil
}
