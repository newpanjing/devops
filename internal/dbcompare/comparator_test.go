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

func TestCompareSchemasDetectsIndexDiffs(t *testing.T) {
	source := &Schema{
		Tables: []Table{
			{
				Name: "order_item",
				Columns: []Column{
					{Name: "id", Type: "bigint", PrimaryKey: true},
					{Name: "order_id", Type: "bigint"},
					{Name: "sku", Type: "varchar(64)"},
				},
				Indexes: []Index{
					{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
					{Name: "idx_order_id", Columns: []string{"order_id"}},
					{Name: "idx_sku", Unique: true, Columns: []string{"sku"}},
				},
			},
		},
	}
	target := &Schema{
		Tables: []Table{
			{
				Name: "order_item",
				Columns: []Column{
					{Name: "id", Type: "bigint", PrimaryKey: true},
					{Name: "order_id", Type: "bigint"},
					{Name: "sku", Type: "varchar(64)"},
				},
				Indexes: []Index{
					{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
					{Name: "idx_sku", Columns: []string{"sku"}},
					{Name: "idx_legacy", Columns: []string{"order_id"}},
				},
			},
		},
	}

	diff := CompareSchemas(source, target)
	if len(diff.TableDiffs) != 1 {
		t.Fatalf("expected 1 table diff, got %d", len(diff.TableDiffs))
	}
	tableDiff := diff.TableDiffs[0]

	var creates, alters, drops int
	for _, indexDiff := range tableDiff.IndexDiffs {
		switch indexDiff.Type {
		case DiffTypeCreate:
			creates++
			if indexDiff.IndexName != "idx_order_id" {
				t.Fatalf("expected create idx_order_id, got %s", indexDiff.IndexName)
			}
		case DiffTypeAlter:
			alters++
			if indexDiff.IndexName != "idx_sku" {
				t.Fatalf("expected alter idx_sku, got %s", indexDiff.IndexName)
			}
		case DiffTypeDrop:
			drops++
			if indexDiff.IndexName != "idx_legacy" {
				t.Fatalf("expected drop idx_legacy, got %s", indexDiff.IndexName)
			}
		}
	}
	if creates != 1 || alters != 1 || drops != 1 {
		t.Fatalf("expected 1 create/1 alter/1 drop index diff, got %d/%d/%d", creates, alters, drops)
	}
}

func TestCompareSchemasDetectsForeignKeyDiffs(t *testing.T) {
	source := &Schema{
		Tables: []Table{
			{
				Name: "order_item",
				Columns: []Column{
					{Name: "id", Type: "bigint", PrimaryKey: true},
					{Name: "order_id", Type: "bigint"},
				},
				ForeignKeys: []ForeignKey{
					// 目标库存在相同关系但约束名自动生成且级联规则不同 -> ALTER
					{Name: "fk_order_item_order", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE", OnUpdate: "RESTRICT"},
					// 目标库缺失此外键 -> CREATE
					{Name: "fk_order_item_user", FromColumns: []string{"user_id"}, ToTable: "user", ToColumns: []string{"id"}, OnDelete: "RESTRICT"},
				},
			},
		},
	}
	target := &Schema{
		Tables: []Table{
			{
				Name: "order_item",
				Columns: []Column{
					{Name: "id", Type: "bigint", PrimaryKey: true},
					{Name: "order_id", Type: "bigint"},
				},
				ForeignKeys: []ForeignKey{
					{Name: "order_item_ibfk_1", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "RESTRICT", OnUpdate: "RESTRICT"},
					// 源库已此外键 -> DROP
					{Name: "fk_legacy", FromColumns: []string{"legacy_id"}, ToTable: "legacy", ToColumns: []string{"id"}},
				},
			},
		},
	}

	diff := CompareSchemas(source, target)
	if len(diff.TableDiffs) != 1 {
		t.Fatalf("expected 1 table diff, got %d", len(diff.TableDiffs))
	}
	tableDiff := diff.TableDiffs[0]

	var creates, alters, drops int
	for _, fkDiff := range tableDiff.ForeignKeyDiffs {
		switch fkDiff.Type {
		case DiffTypeCreate:
			creates++
		case DiffTypeAlter:
			alters++
			if fkDiff.TargetForeignKey == nil || fkDiff.TargetForeignKey.Name != "order_item_ibfk_1" {
				t.Fatalf("expected alter diff to carry target constraint name, got %+v", fkDiff.TargetForeignKey)
			}
		case DiffTypeDrop:
			drops++
		}
	}
	if creates != 1 || alters != 1 || drops != 1 {
		t.Fatalf("expected 1 create/1 alter/1 drop foreign key diff, got %d/%d/%d", creates, alters, drops)
	}
}

func TestCompareSchemasIgnoresIdenticalIndexesAndForeignKeys(t *testing.T) {
	source := &Schema{
		Tables: []Table{
			{
				Name: "order_item",
				Columns: []Column{
					{Name: "id", Type: "bigint", PrimaryKey: true},
					{Name: "order_id", Type: "bigint"},
				},
				Indexes: []Index{
					{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
					{Name: "idx_order_id", Columns: []string{"order_id"}},
				},
				ForeignKeys: []ForeignKey{
					{Name: "fk_a", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE"},
				},
			},
		},
	}
	target := &Schema{
		Tables: []Table{
			{
				Name: "order_item",
				Columns: []Column{
					{Name: "id", Type: "bigint", PrimaryKey: true},
					{Name: "order_id", Type: "bigint"},
				},
				Indexes: []Index{
					{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
					{Name: "idx_order_id", Columns: []string{"order_id"}},
				},
				ForeignKeys: []ForeignKey{
					{Name: "fk_different_auto_name", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE"},
				},
			},
		},
	}

	diff := CompareSchemas(source, target)
	if len(diff.TableDiffs) != 0 {
		t.Fatalf("expected no table diff for identical indexes/foreign keys, got %d", len(diff.TableDiffs))
	}
}
