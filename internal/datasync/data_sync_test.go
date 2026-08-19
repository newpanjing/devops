package datasync

import (
	"strings"
	"testing"

	"dbsync/internal/dbcompare"
)

// TestComparisonFieldIgnoresID 验证 id 变化不会产生差异，SQL 使用自定义字段定位数据。
func TestComparisonFieldIgnoresID(t *testing.T) {
	source := map[string]interface{}{"id": 1, "code": "A001", "name": "新名称"}
	target := map[string]interface{}{"id": 2, "code": "A001", "name": "新名称"}
	if !rowsEqual(source, target) {
		t.Fatal("id difference must be ignored")
	}

	table := &dbcompare.Table{Columns: []dbcompare.Column{{Name: "id"}, {Name: "code"}, {Name: "name"}}}
	updateSQL := buildUpdateSQL(table, source, "code")
	if !strings.Contains(updateSQL, "WHERE `code` = 'A001'") || strings.Contains(updateSQL, "`id`") {
		t.Fatalf("unexpected update SQL: %s", updateSQL)
	}
	insertSQL := buildInsertSQL(table, source)
	if strings.Contains(insertSQL, "`id`") {
		t.Fatalf("unexpected insert SQL: %s", insertSQL)
	}
}
