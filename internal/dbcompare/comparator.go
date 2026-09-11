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
	// TableDiff 表示当前表完成比对后产生的差异，供流式调用方即时生成 SQL。
	TableDiff *TableDiff
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
						tableDiff := compareTablesWithProgress(sourceTable, targetTable, progressFunc)
						if tableDiff != nil {
							notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{TableName: sourceTable.Name, Message: "已完成表比对", TableDiff: tableDiff})
						}
						results <- tableCompareResult{
							index:     tableIndex,
							tableDiff: tableDiff,
						}
						continue
					}

					tableDiff := &TableDiff{Type: DiffTypeCreate, TableName: sourceTable.Name, SourceTable: sourceTable}
					notifySchemaCompareProgress(progressFunc, SchemaCompareProgress{
						TableName: sourceTable.Name,
						Message:   "已完成表比对，目标数据库缺少该表",
						TableDiff: tableDiff,
					})
					results <- tableCompareResult{
						index:     tableIndex,
						tableDiff: tableDiff,
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
				TableDiff: &TableDiff{Type: DiffTypeDrop, TableName: targetTable.Name, TargetTable: targetTable},
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

	diff.IndexDiffs = compareIndexes(source.Indexes, target.Indexes)
	diff.ForeignKeyDiffs = compareForeignKeys(source.ForeignKeys, target.ForeignKeys)

	if len(diff.ColumnDiffs) == 0 && len(diff.IndexDiffs) == 0 && len(diff.ForeignKeyDiffs) == 0 && !diff.TableCommentChanged {
		return nil
	}

	return diff
}

// primaryKeyIndexName 是 MySQL 主键在 SHOW INDEX 中使用的索引名，主键已通过字段定义生成，需要跳过。
const primaryKeyIndexName = "PRIMARY"

func compareIndexes(sourceIndexes, targetIndexes []Index) []IndexDiff {
	targetIndexMap := make(map[string]*Index)
	for i := range targetIndexes {
		if strings.EqualFold(targetIndexes[i].Name, primaryKeyIndexName) {
			continue
		}
		targetIndexMap[targetIndexes[i].Name] = &targetIndexes[i]
	}
	sourceIndexMap := make(map[string]*Index)
	for i := range sourceIndexes {
		if strings.EqualFold(sourceIndexes[i].Name, primaryKeyIndexName) {
			continue
		}
		sourceIndexMap[sourceIndexes[i].Name] = &sourceIndexes[i]
	}

	var diffs []IndexDiff
	for sourceIndexIndex := range sourceIndexes {
		sourceIndex := &sourceIndexes[sourceIndexIndex]
		if strings.EqualFold(sourceIndex.Name, primaryKeyIndexName) {
			continue
		}
		if targetIndex, ok := targetIndexMap[sourceIndex.Name]; ok {
			if !indexesEqual(sourceIndex, targetIndex) {
				diffs = append(diffs, IndexDiff{
					Type:        DiffTypeAlter,
					IndexName:   sourceIndex.Name,
					SourceIndex: sourceIndex,
					TargetIndex: targetIndex,
				})
			}
		} else {
			diffs = append(diffs, IndexDiff{
				Type:        DiffTypeCreate,
				IndexName:   sourceIndex.Name,
				SourceIndex: sourceIndex,
			})
		}
	}

	for targetIndexIndex := range targetIndexes {
		targetIndex := &targetIndexes[targetIndexIndex]
		if strings.EqualFold(targetIndex.Name, primaryKeyIndexName) {
			continue
		}
		if _, exists := sourceIndexMap[targetIndex.Name]; !exists {
			diffs = append(diffs, IndexDiff{
				Type:        DiffTypeDrop,
				IndexName:   targetIndex.Name,
				TargetIndex: targetIndex,
			})
		}
	}

	return diffs
}

func indexesEqual(a, b *Index) bool {
	if a.Unique != b.Unique || len(a.Columns) != len(b.Columns) {
		return false
	}
	for i := range a.Columns {
		if !strings.EqualFold(a.Columns[i], b.Columns[i]) {
			return false
		}
	}
	return true
}

func compareForeignKeys(sourceForeignKeys, targetForeignKeys []ForeignKey) []ForeignKeyDiff {
	targetFKMap := make(map[string]*ForeignKey)
	for i := range targetForeignKeys {
		targetFKMap[foreignKeyIdentity(&targetForeignKeys[i])] = &targetForeignKeys[i]
	}
	sourceFKMap := make(map[string]*ForeignKey)
	for i := range sourceForeignKeys {
		sourceFKMap[foreignKeyIdentity(&sourceForeignKeys[i])] = &sourceForeignKeys[i]
	}

	var diffs []ForeignKeyDiff
	for sourceFKIndex := range sourceForeignKeys {
		sourceFK := &sourceForeignKeys[sourceFKIndex]
		if targetFK, ok := targetFKMap[foreignKeyIdentity(sourceFK)]; ok {
			if !foreignKeysEqual(sourceFK, targetFK) {
				diffs = append(diffs, ForeignKeyDiff{
					Type:             DiffTypeAlter,
					ForeignKeyName:   sourceFK.Name,
					SourceForeignKey: sourceFK,
					TargetForeignKey: targetFK,
				})
			}
		} else {
			diffs = append(diffs, ForeignKeyDiff{
				Type:             DiffTypeCreate,
				ForeignKeyName:   sourceFK.Name,
				SourceForeignKey: sourceFK,
			})
		}
	}

	for targetFKIndex := range targetForeignKeys {
		targetFK := &targetForeignKeys[targetFKIndex]
		if _, exists := sourceFKMap[foreignKeyIdentity(targetFK)]; !exists {
			diffs = append(diffs, ForeignKeyDiff{
				Type:             DiffTypeDrop,
				ForeignKeyName:   targetFK.Name,
				TargetForeignKey: targetFK,
			})
		}
	}

	return diffs
}

// foreignKeyIdentity 以外键关系（本端列、引用表、引用列）作为匹配依据，
// 避免源库和目标库自动生成的约束名不同（如 MySQL 的 tbl_ibfk_N）导致误报。
func foreignKeyIdentity(fk *ForeignKey) string {
	return strings.ToLower(strings.Join(foreignKeyFromColumns(fk), ",") + "->" + fk.ToTable + "(" + strings.Join(foreignKeyToColumns(fk), ",") + ")")
}

func foreignKeysEqual(a, b *ForeignKey) bool {
	return normalizeForeignKeyRule(a.OnDelete) == normalizeForeignKeyRule(b.OnDelete) &&
		normalizeForeignKeyRule(a.OnUpdate) == normalizeForeignKeyRule(b.OnUpdate)
}

// normalizeForeignKeyRule 统一外键级联规则的写法，空值按默认 RESTRICT 处理。
func normalizeForeignKeyRule(rule string) string {
	normalized := strings.ToUpper(strings.TrimSpace(rule))
	if normalized == "" {
		return "RESTRICT"
	}
	return normalized
}

func foreignKeyFromColumns(fk *ForeignKey) []string {
	if len(fk.FromColumns) > 0 {
		return fk.FromColumns
	}
	if fk.FromColumn != "" {
		return []string{fk.FromColumn}
	}
	return nil
}

func foreignKeyToColumns(fk *ForeignKey) []string {
	if len(fk.ToColumns) > 0 {
		return fk.ToColumns
	}
	if fk.ToColumn != "" {
		return []string{fk.ToColumn}
	}
	return nil
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
