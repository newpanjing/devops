package dbcompare

import (
	"strings"
	"sync"
)

func CompareSchemas(source, target *Schema) *SchemaDiff {
	return CompareSchemasWithProgress(source, target, 1, nil)
}

type SchemaCompareProgress struct {
	TableName  string
	ColumnName string
	Current    int
	Total      int
	Message    string
}

type SchemaCompareProgressFunc func(progress SchemaCompareProgress)

func CompareSchemasWithProgress(source, target *Schema, workerCount int, progressFunc SchemaCompareProgressFunc) *SchemaDiff {
	diff := &SchemaDiff{}

	sourceTables := make(map[string]*Table)
	for i := range source.Tables {
		sourceTables[source.Tables[i].Name] = &source.Tables[i]
	}

	targetTables := make(map[string]*Table)
	for i := range target.Tables {
		targetTables[target.Tables[i].Name] = &target.Tables[i]
	}

	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(source.Tables) {
		workerCount = len(source.Tables)
	}

	type tableCompareResult struct {
		index     int
		tableDiff *TableDiff
	}

	if workerCount > 0 {
		tasks := make(chan int, len(source.Tables))
		results := make(chan tableCompareResult, len(source.Tables))
		var waitGroup sync.WaitGroup

		for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				for tableIndex := range tasks {
					sourceTable := &source.Tables[tableIndex]
					notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
						TableName: sourceTable.Name,
						Message:   "正在比对表",
					})

					if targetTable, exists := targetTables[sourceTable.Name]; exists {
						results <- tableCompareResult{
							index:     tableIndex,
							tableDiff: compareTablesWithProgress(sourceTable, targetTable, progressFunc),
						}
						continue
					}

					notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
						TableName: sourceTable.Name,
						Message:   "目标数据库缺少该表，标记为新增",
					})
					results <- tableCompareResult{
						index: tableIndex,
						tableDiff: &TableDiff{
							Type:        DiffTypeCreate,
							TableName:   sourceTable.Name,
							SourceTable: sourceTable,
						},
					}
				}
			}()
		}

		for tableIndex := range source.Tables {
			tasks <- tableIndex
		}
		close(tasks)

		go func() {
			waitGroup.Wait()
			close(results)
		}()

		orderedTableDiffs := make([]*TableDiff, len(source.Tables))
		for result := range results {
			orderedTableDiffs[result.index] = result.tableDiff
		}
		for _, tableDiff := range orderedTableDiffs {
			if tableDiff != nil {
				diff.TableDiffs = append(diff.TableDiffs, *tableDiff)
			}
		}
	}

	for targetTableIndex := range target.Tables {
		targetTable := &target.Tables[targetTableIndex]
		if _, exists := sourceTables[targetTable.Name]; !exists {
			notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
				TableName: targetTable.Name,
				Message:   "源数据库缺少该表，标记为删除",
			})
			diff.TableDiffs = append(diff.TableDiffs, TableDiff{
				Type:        DiffTypeDrop,
				TableName:   targetTable.Name,
				TargetTable: targetTable,
			})
		}
	}

	return diff
}

func compareTables(source, target *Table) *TableDiff {
	return compareTablesWithProgress(source, target, nil)
}

func compareTablesWithProgress(source, target *Table, progressFunc SchemaCompareProgressFunc) *TableDiff {
	diff := &TableDiff{
		Type:                DiffTypeAlter,
		TableName:           source.Name,
		SourceTable:         source,
		TargetTable:         target,
		TableCommentChanged: source.Comment != target.Comment,
	}

	if diff.TableCommentChanged {
		notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
			TableName: source.Name,
			Message:   "表备注存在差异，标记为修改",
		})
	}

	sourceColumns := make(map[string]*Column)
	for i := range source.Columns {
		sourceColumns[source.Columns[i].Name] = &source.Columns[i]
	}

	targetColumns := make(map[string]*Column)
	for i := range target.Columns {
		targetColumns[target.Columns[i].Name] = &target.Columns[i]
	}

	for sourceColumnIndex := range source.Columns {
		sourceCol := &source.Columns[sourceColumnIndex]
		notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
			TableName:  source.Name,
			ColumnName: sourceCol.Name,
			Message:    "正在比对字段",
		})
		if targetCol, ok := targetColumns[sourceCol.Name]; ok {
			if !columnsEqual(sourceCol, targetCol) {
				diff.ColumnDiffs = append(diff.ColumnDiffs, ColumnDiff{
					Type:         DiffTypeAlter,
					ColumnName:   sourceCol.Name,
					SourceColumn: sourceCol,
					TargetColumn: targetCol,
				})
				notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
					TableName:  source.Name,
					ColumnName: sourceCol.Name,
					Message:    "字段存在差异，标记为修改",
				})
			}
		} else {
			diff.ColumnDiffs = append(diff.ColumnDiffs, ColumnDiff{
				Type:         DiffTypeCreate,
				ColumnName:   sourceCol.Name,
				SourceColumn: sourceCol,
			})
			notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
				TableName:  source.Name,
				ColumnName: sourceCol.Name,
				Message:    "目标数据库缺少字段，标记为新增",
			})
		}
	}

	for targetColumnIndex := range target.Columns {
		targetCol := &target.Columns[targetColumnIndex]
		if _, exists := sourceColumns[targetCol.Name]; !exists {
			diff.ColumnDiffs = append(diff.ColumnDiffs, ColumnDiff{
				Type:         DiffTypeDrop,
				ColumnName:   targetCol.Name,
				TargetColumn: targetCol,
			})
			notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
				TableName:  source.Name,
				ColumnName: targetCol.Name,
				Message:    "源数据库缺少字段，标记为删除",
			})
		}
	}

	if len(diff.ColumnDiffs) == 0 && !diff.TableCommentChanged {
		return nil
	}

	return diff
}

func notifySchemaCompareProgress(progressFunc SchemaCompareProgressFunc, progress SchemaCompareProgress) {
	if progressFunc != nil {
		progressFunc(progress)
	}
}

func columnsEqual(a, b *Column) bool {
	return a.Name == b.Name &&
		strings.EqualFold(a.Type, b.Type) &&
		a.Nullable == b.Nullable &&
		a.DefaultValue == b.DefaultValue &&
		a.Comment == b.Comment &&
		a.PrimaryKey == b.PrimaryKey &&
		a.AutoIncrement == b.AutoIncrement
}
