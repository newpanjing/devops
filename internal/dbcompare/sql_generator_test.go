package dbcompare

import (
	"strings"
	"testing"
)

func TestGenerateMySQLCreateTableSQLUsesSeparatePrimaryKey(t *testing.T) {
	diff := &SchemaDiff{
		TableDiffs: []TableDiff{
			{
				Type:      DiffTypeCreate,
				TableName: "invite_register_log",
				SourceTable: &Table{
					Name: "invite_register_log",
					Columns: []Column{
						{Name: "id", Type: "bigint", Comment: "主键ID", PrimaryKey: true, AutoIncrement: true},
						{Name: "email", Type: "varchar(255)", Comment: "邮箱", Nullable: false},
					},
					Comment: "邀请注册日志",
				},
			},
		},
	}

	sqls := GenerateSQL(diff, sqlDatabaseTypeMySQL)
	if len(sqls) != 1 {
		t.Fatalf("expected 1 sql statement, got %d", len(sqls))
	}

	sql := sqls[0]
	if strings.Contains(sql, "`id` bigint PRIMARY KEY AUTO_INCREMENT") {
		t.Fatalf("generated invalid inline primary key order: %s", sql)
	}
	if !strings.Contains(sql, "`id` bigint NOT NULL AUTO_INCREMENT") {
		t.Fatalf("expected id column to be NOT NULL AUTO_INCREMENT: %s", sql)
	}
	if !strings.Contains(sql, "PRIMARY KEY (`id`)") {
		t.Fatalf("expected separate primary key definition: %s", sql)
	}
	if !strings.Contains(sql, "COMMENT '主键ID'") || !strings.Contains(sql, "COMMENT='邀请注册日志'") {
		t.Fatalf("expected column and table comments: %s", sql)
	}
}

func TestGenerateMySQLCreateTableWithIndexesAndForeignKeys(t *testing.T) {
	diff := &SchemaDiff{
		TableDiffs: []TableDiff{
			{
				Type:      DiffTypeCreate,
				TableName: "order_item",
				SourceTable: &Table{
					Name: "order_item",
					Columns: []Column{
						{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true},
						{Name: "order_id", Type: "bigint", Nullable: false},
					},
					Indexes: []Index{
						{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
						{Name: "idx_order_id", Columns: []string{"order_id"}},
						{Name: "uk_order_sku", Unique: true, Columns: []string{"order_id", "sku"}},
					},
					ForeignKeys: []ForeignKey{
						{Name: "fk_order_item_order", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE", OnUpdate: "RESTRICT"},
					},
				},
			},
		},
	}

	sqls := GenerateSQL(diff, sqlDatabaseTypeMySQL)
	if len(sqls) != 4 {
		t.Fatalf("expected 4 sql statements (create table + 2 indexes + 1 fk), got %d: %v", len(sqls), sqls)
	}

	fullSQL := strings.Join(sqls, "\n")
	if !strings.Contains(fullSQL, "CREATE TABLE IF NOT EXISTS `order_item`") {
		t.Fatalf("expected create table statement: %s", fullSQL)
	}
	if !strings.Contains(sqls[1], "ALTER TABLE `order_item` ADD INDEX `idx_order_id` (`order_id`)") {
		t.Fatalf("expected add index statement, got %s", sqls[1])
	}
	if !strings.Contains(sqls[2], "ALTER TABLE `order_item` ADD UNIQUE INDEX `uk_order_sku` (`order_id`, `sku`)") {
		t.Fatalf("expected add unique index statement, got %s", sqls[2])
	}
	// 外键语句应放在最后
	if !strings.Contains(sqls[3], "ALTER TABLE `order_item` ADD CONSTRAINT `fk_order_item_order` FOREIGN KEY (`order_id`) REFERENCES `order` (`id`) ON DELETE CASCADE ON UPDATE RESTRICT") {
		t.Fatalf("expected add foreign key statement at end, got %s", sqls[3])
	}
}

func TestGenerateMySQLAlterTableIndexesAndForeignKeys(t *testing.T) {
	diff := &SchemaDiff{
		TableDiffs: []TableDiff{
			{
				Type:      DiffTypeAlter,
				TableName: "order_item",
				SourceTable: &Table{
					Name: "order_item",
					Indexes: []Index{
						{Name: "idx_order_id", Unique: true, Columns: []string{"order_id"}},
					},
					ForeignKeys: []ForeignKey{
						{Name: "fk_new", FromColumns: []string{"user_id"}, ToTable: "user", ToColumns: []string{"id"}, OnDelete: "CASCADE"},
						{Name: "fk_order", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE"},
					},
				},
				TargetTable: &Table{Name: "order_item"},
				IndexDiffs: []IndexDiff{
					{
						Type:        DiffTypeAlter,
						IndexName:   "idx_order_id",
						SourceIndex: &Index{Name: "idx_order_id", Unique: true, Columns: []string{"order_id"}},
						TargetIndex: &Index{Name: "idx_order_id", Columns: []string{"order_id"}},
					},
					{
						Type:        DiffTypeDrop,
						IndexName:   "idx_legacy",
						TargetIndex: &Index{Name: "idx_legacy", Columns: []string{"legacy_id"}},
					},
				},
				ForeignKeyDiffs: []ForeignKeyDiff{
					{
						Type:             DiffTypeCreate,
						ForeignKeyName:   "fk_new",
						SourceForeignKey: &ForeignKey{Name: "fk_new", FromColumns: []string{"user_id"}, ToTable: "user", ToColumns: []string{"id"}, OnDelete: "CASCADE"},
					},
					{
						Type:             DiffTypeAlter,
						ForeignKeyName:   "fk_order",
						SourceForeignKey: &ForeignKey{Name: "fk_order", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE"},
						TargetForeignKey: &ForeignKey{Name: "order_item_ibfk_1", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "RESTRICT"},
					},
				},
			},
		},
	}

	sqls := GenerateSQL(diff, sqlDatabaseTypeMySQL)

	// 期望顺序：删除旧外键 -> 删除旧索引 -> 重建索引 -> 最后新增外键
	expected := []string{
		"ALTER TABLE `order_item` DROP FOREIGN KEY `order_item_ibfk_1`;",
		"ALTER TABLE `order_item` DROP INDEX `idx_order_id`;",
		"ALTER TABLE `order_item` DROP INDEX `idx_legacy`;",
		"ALTER TABLE `order_item` ADD UNIQUE INDEX `idx_order_id` (`order_id`);",
		"ALTER TABLE `order_item` ADD CONSTRAINT `fk_new` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE;",
		"ALTER TABLE `order_item` ADD CONSTRAINT `fk_order` FOREIGN KEY (`order_id`) REFERENCES `order` (`id`) ON DELETE CASCADE;",
	}
	if len(sqls) != len(expected) {
		t.Fatalf("expected %d sql statements, got %d: %v", len(expected), len(sqls), sqls)
	}
	for i, want := range expected {
		if sqls[i] != want {
			t.Fatalf("statement %d mismatch:\nwant: %s\ngot:  %s", i, want, sqls[i])
		}
	}
}

func TestGeneratePostgresIndexAndForeignKeySQL(t *testing.T) {
	diff := &SchemaDiff{
		TableDiffs: []TableDiff{
			{
				Type:      DiffTypeAlter,
				TableName: "order_item",
				SourceTable: &Table{
					Name:        "order_item",
					Indexes:     []Index{{Name: "idx_order_id", Columns: []string{"order_id"}}},
					ForeignKeys: []ForeignKey{{Name: "fk_order", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE"}},
				},
				TargetTable: &Table{Name: "order_item"},
				IndexDiffs: []IndexDiff{
					{Type: DiffTypeCreate, IndexName: "idx_order_id", SourceIndex: &Index{Name: "idx_order_id", Columns: []string{"order_id"}}},
					{Type: DiffTypeDrop, IndexName: "idx_old", TargetIndex: &Index{Name: "idx_old", Columns: []string{"x"}}},
				},
				ForeignKeyDiffs: []ForeignKeyDiff{
					{Type: DiffTypeDrop, ForeignKeyName: "fk_old", TargetForeignKey: &ForeignKey{Name: "fk_old", FromColumns: []string{"x"}, ToTable: "y", ToColumns: []string{"id"}}},
					{Type: DiffTypeCreate, ForeignKeyName: "fk_order", SourceForeignKey: &ForeignKey{Name: "fk_order", FromColumns: []string{"order_id"}, ToTable: "order", ToColumns: []string{"id"}, OnDelete: "CASCADE"}},
				},
			},
		},
	}

	sqls := GenerateSQL(diff, sqlDatabaseTypePostgres)
	fullSQL := strings.Join(sqls, "\n")
	if !strings.Contains(fullSQL, "ALTER TABLE \"order_item\" DROP CONSTRAINT \"fk_old\";") {
		t.Fatalf("expected postgres drop constraint statement: %s", fullSQL)
	}
	if !strings.Contains(fullSQL, "DROP INDEX IF EXISTS \"idx_old\";") {
		t.Fatalf("expected postgres drop index statement: %s", fullSQL)
	}
	if !strings.Contains(fullSQL, "CREATE INDEX \"idx_order_id\" ON \"order_item\" (\"order_id\");") {
		t.Fatalf("expected postgres create index statement: %s", fullSQL)
	}
	if !strings.Contains(fullSQL, "ALTER TABLE \"order_item\" ADD CONSTRAINT \"fk_order\" FOREIGN KEY (\"order_id\") REFERENCES \"order\" (\"id\") ON DELETE CASCADE;") {
		t.Fatalf("expected postgres add foreign key statement: %s", fullSQL)
	}
}

func TestGenerateMySQLAlterTableCommentSQL(t *testing.T) {
	diff := &SchemaDiff{
		TableDiffs: []TableDiff{
			{
				Type:                DiffTypeAlter,
				TableName:           "invite_register_log",
				TableCommentChanged: true,
				SourceTable: &Table{
					Name:    "invite_register_log",
					Comment: "邀请注册日志",
				},
				TargetTable: &Table{
					Name:    "invite_register_log",
					Comment: "旧备注",
				},
			},
		},
	}

	sqls := GenerateSQL(diff, sqlDatabaseTypeMySQL)
	if len(sqls) != 1 {
		t.Fatalf("expected 1 sql statement, got %d", len(sqls))
	}
	if sqls[0] != "ALTER TABLE `invite_register_log` COMMENT = '邀请注册日志';" {
		t.Fatalf("unexpected table comment sql: %s", sqls[0])
	}
}
