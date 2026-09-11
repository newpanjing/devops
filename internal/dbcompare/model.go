package dbcompare

type Column struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Comment       string `json:"comment"`
	Nullable      bool   `json:"nullable"`
	DefaultValue  string `json:"default_value"`
	PrimaryKey    bool   `json:"primary_key"`
	AutoIncrement bool   `json:"auto_increment"`
}

type Index struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Column  string   `json:"column"`
	Columns []string `json:"columns"`
}

type ForeignKey struct {
	Name        string   `json:"name"`
	FromColumn  string   `json:"from_column"`
	FromColumns []string `json:"from_columns,omitempty"`
	ToTable     string   `json:"to_table"`
	ToColumn    string   `json:"to_column"`
	ToColumns   []string `json:"to_columns,omitempty"`
	OnDelete    string   `json:"on_delete"`
	OnUpdate    string   `json:"on_update"`
}

type Table struct {
	Name        string       `json:"name"`
	Comment     string       `json:"comment"`
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes"`
	ForeignKeys []ForeignKey `json:"foreign_keys"`
}

type Schema struct {
	Tables []Table `json:"tables"`
}

type DiffType string

const (
	DiffTypeCreate DiffType = "CREATE"
	DiffTypeAlter  DiffType = "ALTER"
	DiffTypeDrop   DiffType = "DROP"
)

type ColumnDiff struct {
	Type         DiffType `json:"type"`
	ColumnName   string   `json:"column_name"`
	SourceColumn *Column  `json:"source_column"`
	TargetColumn *Column  `json:"target_column"`
}

type IndexDiff struct {
	Type        DiffType `json:"type"`
	IndexName   string   `json:"index_name"`
	SourceIndex *Index   `json:"source_index"`
	TargetIndex *Index   `json:"target_index"`
}

type ForeignKeyDiff struct {
	Type             DiffType    `json:"type"`
	ForeignKeyName   string      `json:"foreign_key_name"`
	SourceForeignKey *ForeignKey `json:"source_foreign_key"`
	TargetForeignKey *ForeignKey `json:"target_foreign_key"`
}

type TableDiff struct {
	Type                DiffType         `json:"type"`
	TableName           string           `json:"table_name"`
	SourceTable         *Table           `json:"source_table"`
	TargetTable         *Table           `json:"target_table"`
	TableCommentChanged bool             `json:"table_comment_changed"`
	ColumnDiffs         []ColumnDiff     `json:"column_diffs"`
	IndexDiffs          []IndexDiff      `json:"index_diffs"`
	ForeignKeyDiffs     []ForeignKeyDiff `json:"foreign_key_diffs"`
}

type SchemaDiff struct {
	TableDiffs []TableDiff `json:"table_diffs"`
}
