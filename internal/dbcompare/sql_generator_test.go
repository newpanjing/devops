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
