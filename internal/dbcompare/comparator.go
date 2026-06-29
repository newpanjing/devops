package dbcompare

import (
	"strings"
)

func CompareSchemas(source, target *Schema) *SchemaDiff {
	diff := &SchemaDiff{}

	sourceTables := make(map[string]*Table)
	for i := range source.Tables {
		sourceTables[source.Tables[i].Name] = &source.Tables[i]
	}

	targetTables := make(map[string]*Table)
	for i := range target.Tables {
		targetTables[target.Tables[i].Name] = &target.Tables[i]
	}

	for name, sourceTable := range sourceTables {
		if targetTable, ok := targetTables[name]; ok {
			tableDiff := compareTables(sourceTable, targetTable)
			if tableDiff != nil {
				diff.TableDiffs = append(diff.TableDiffs, *tableDiff)
			}
		} else {
			diff.TableDiffs = append(diff.TableDiffs, TableDiff{
				Type:        DiffTypeCreate,
				TableName:   name,
				SourceTable: sourceTable,
				TargetTable: nil,
			})
		}
	}

	for name, targetTable := range targetTables {
		if _, ok := sourceTables[name]; !ok {
			diff.TableDiffs = append(diff.TableDiffs, TableDiff{
				Type:        DiffTypeDrop,
				TableName:   name,
				SourceTable: nil,
				TargetTable: targetTable,
			})
		}
	}

	return diff
}

func compareTables(source, target *Table) *TableDiff {
	diff := &TableDiff{
		Type:        DiffTypeAlter,
		TableName:   source.Name,
		SourceTable: source,
		TargetTable: target,
	}

	sourceColumns := make(map[string]*Column)
	for i := range source.Columns {
		sourceColumns[source.Columns[i].Name] = &source.Columns[i]
	}

	targetColumns := make(map[string]*Column)
	for i := range target.Columns {
		targetColumns[target.Columns[i].Name] = &target.Columns[i]
	}

	for name, sourceCol := range sourceColumns {
		if targetCol, ok := targetColumns[name]; ok {
			if !columnsEqual(sourceCol, targetCol) {
				diff.ColumnDiffs = append(diff.ColumnDiffs, ColumnDiff{
					Type:          DiffTypeAlter,
					ColumnName:    name,
					SourceColumn:  sourceCol,
					TargetColumn:  targetCol,
				})
			}
		} else {
			diff.ColumnDiffs = append(diff.ColumnDiffs, ColumnDiff{
				Type:          DiffTypeCreate,
				ColumnName:    name,
				SourceColumn:  sourceCol,
				TargetColumn:  nil,
			})
		}
	}

	for name, targetCol := range targetColumns {
		if _, ok := sourceColumns[name]; !ok {
			diff.ColumnDiffs = append(diff.ColumnDiffs, ColumnDiff{
				Type:          DiffTypeDrop,
				ColumnName:    name,
				SourceColumn:  nil,
				TargetColumn:  targetCol,
			})
		}
	}

	if len(diff.ColumnDiffs) == 0 {
		return nil
	}

	return diff
}

func columnsEqual(a, b *Column) bool {
	return a.Name == b.Name &&
		strings.EqualFold(a.Type, b.Type) &&
		a.Nullable == b.Nullable &&
		a.DefaultValue == b.DefaultValue &&
		a.PrimaryKey == b.PrimaryKey &&
		a.AutoIncrement == b.AutoIncrement
}
