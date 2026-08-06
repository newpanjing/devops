package dbcompare

import "testing"

func TestCompareSchemasDetectsTableAndColumnComments(t *testing.T) {
	source := &Schema{
		Tables: []Table{
			{
				Name:    "company",
				Comment: "公司表",
				Columns: []Column{
					{Name: "id", Type: "bigint", Comment: "主键ID", PrimaryKey: true},
				},
			},
		},
	}
	target := &Schema{
		Tables: []Table{
			{
				Name:    "company",
				Comment: "旧公司表",
				Columns: []Column{
					{Name: "id", Type: "bigint", Comment: "旧主键ID", PrimaryKey: true},
				},
			},
		},
	}

	diff := CompareSchemas(source, target)
	if len(diff.TableDiffs) != 1 {
		t.Fatalf("expected 1 table diff, got %d", len(diff.TableDiffs))
	}
	tableDiff := diff.TableDiffs[0]
	if !tableDiff.TableCommentChanged {
		t.Fatal("expected table comment change to be detected")
	}
	if len(tableDiff.ColumnDiffs) != 1 {
		t.Fatalf("expected 1 column diff, got %d", len(tableDiff.ColumnDiffs))
	}
}
