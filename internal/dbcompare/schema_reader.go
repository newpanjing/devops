package dbcompare

import (
	"database/sql"
	"fmt"
	"strings"
)

func (dc *DBConnection) ReadSchema() (*Schema, error) {
	switch dc.DBType {
	case "mysql":
		return dc.readMySQLSchema()
	case "postgres":
		return dc.readPostgresSchema()
	case "sqlite":
		return dc.readSQLiteSchema()
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dc.DBType)
	}
}

func (dc *DBConnection) readMySQLSchema() (*Schema, error) {
	rows, err := dc.DB.Query("SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		table, err := dc.readMySQLTable(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to read table %s: %w", tableName, err)
		}
		tables = append(tables, table)
	}

	return &Schema{Tables: tables}, nil
}

func (dc *DBConnection) readMySQLTable(tableName string) (Table, error) {
	columns, err := dc.readMySQLColumns(tableName)
	if err != nil {
		return Table{}, err
	}

	indexes, err := dc.readMySQLIndexes(tableName)
	if err != nil {
		return Table{}, err
	}

	foreignKeys, err := dc.readMySQLForeignKeys(tableName)
	if err != nil {
		return Table{}, err
	}

	return Table{
		Name:        tableName,
		Columns:     columns,
		Indexes:     indexes,
		ForeignKeys: foreignKeys,
	}, nil
}

func (dc *DBConnection) readMySQLColumns(tableName string) ([]Column, error) {
	rows, err := dc.DB.Query(fmt.Sprintf("SHOW COLUMNS FROM `%s`", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var name, colType, null, key, extra string
		var defaultValue sql.NullString
		if err := rows.Scan(&name, &colType, &null, &key, &defaultValue, &extra); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		defaultVal := ""
		if defaultValue.Valid {
			defaultVal = defaultValue.String
		}

		columns = append(columns, Column{
			Name:          name,
			Type:          colType,
			Nullable:      null == "YES",
			DefaultValue:  defaultVal,
			PrimaryKey:    key == "PRI",
			AutoIncrement: strings.Contains(extra, "auto_increment"),
		})
	}

	return columns, nil
}

func (dc *DBConnection) readMySQLIndexes(tableName string) ([]Index, error) {
	rows, err := dc.DB.Query(fmt.Sprintf("SHOW INDEX FROM `%s`", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*Index)
	for rows.Next() {
		var table, nonUnique, keyName, seqInIndex, columnName, indexType string
		if err := rows.Scan(&table, &nonUnique, &keyName, &seqInIndex, &columnName, &indexType); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		if _, ok := indexMap[keyName]; !ok {
			indexMap[keyName] = &Index{
				Name:    keyName,
				Unique:  nonUnique == "0",
				Columns: []string{},
			}
		}
		indexMap[keyName].Columns = append(indexMap[keyName].Columns, columnName)
	}

	var indexes []Index
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	return indexes, nil
}

func (dc *DBConnection) readMySQLForeignKeys(tableName string) ([]ForeignKey, error) {
	rows, err := dc.DB.Query(`
		SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME, DELETE_RULE, UPDATE_RULE
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var name, fromCol, toTable, toCol, onDelete, onUpdate string
		if err := rows.Scan(&name, &fromCol, &toTable, &toCol, &onDelete, &onUpdate); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}

		foreignKeys = append(foreignKeys, ForeignKey{
			Name:       name,
			FromColumn: fromCol,
			ToTable:    toTable,
			ToColumn:   toCol,
			OnDelete:   onDelete,
			OnUpdate:   onUpdate,
		})
	}

	return foreignKeys, nil
}

func (dc *DBConnection) readPostgresSchema() (*Schema, error) {
	rows, err := dc.DB.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		table, err := dc.readPostgresTable(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to read table %s: %w", tableName, err)
		}
		tables = append(tables, table)
	}

	return &Schema{Tables: tables}, nil
}

func (dc *DBConnection) readPostgresTable(tableName string) (Table, error) {
	columns, err := dc.readPostgresColumns(tableName)
	if err != nil {
		return Table{}, err
	}

	indexes, err := dc.readPostgresIndexes(tableName)
	if err != nil {
		return Table{}, err
	}

	foreignKeys, err := dc.readPostgresForeignKeys(tableName)
	if err != nil {
		return Table{}, err
	}

	return Table{
		Name:        tableName,
		Columns:     columns,
		Indexes:     indexes,
		ForeignKeys: foreignKeys,
	}, nil
}

func (dc *DBConnection) readPostgresColumns(tableName string) ([]Column, error) {
	rows, err := dc.DB.Query(`
		SELECT column_name, data_type, is_nullable, column_default, is_identity
		FROM information_schema.columns
		WHERE table_name = ? AND table_schema = 'public'
		ORDER BY ordinal_position
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var name, colType, nullable, isIdentity string
		var defaultValue sql.NullString
		if err := rows.Scan(&name, &colType, &nullable, &defaultValue, &isIdentity); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		defaultVal := ""
		if defaultValue.Valid {
			defaultVal = defaultValue.String
		}

		columns = append(columns, Column{
			Name:          name,
			Type:          colType,
			Nullable:      nullable == "YES",
			DefaultValue:  defaultVal,
			PrimaryKey:    false,
			AutoIncrement: isIdentity == "YES",
		})
	}

	pkRows, err := dc.DB.Query(`
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_name = ? AND tc.constraint_type = 'PRIMARY KEY'
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary keys: %w", err)
	}
	defer pkRows.Close()

	for pkRows.Next() {
		var pkCol string
		if err := pkRows.Scan(&pkCol); err != nil {
			return nil, fmt.Errorf("failed to scan primary key: %w", err)
		}
		for i := range columns {
			if columns[i].Name == pkCol {
				columns[i].PrimaryKey = true
				break
			}
		}
	}

	return columns, nil
}

func (dc *DBConnection) readPostgresIndexes(tableName string) ([]Index, error) {
	rows, err := dc.DB.Query(`
		SELECT index_name, is_unique, array_agg(column_name ORDER BY ordinal_position) as columns
		FROM information_schema.statistics
		WHERE table_name = ? AND table_schema = 'public'
		GROUP BY index_name, is_unique
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var name string
		var unique bool
		var columnsStr string
		if err := rows.Scan(&name, &unique, &columnsStr); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		columns := strings.Split(strings.Trim(columnsStr, "{}"), ",")
		for i := range columns {
			columns[i] = strings.TrimSpace(columns[i])
		}

		indexes = append(indexes, Index{
			Name:    name,
			Unique:  unique,
			Columns: columns,
		})
	}

	return indexes, nil
}

func (dc *DBConnection) readPostgresForeignKeys(tableName string) ([]ForeignKey, error) {
	rows, err := dc.DB.Query(`
		SELECT tc.constraint_name, kcu.column_name, ccu.table_name, ccu.column_name, tc.delete_rule, tc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
		ON ccu.constraint_name = tc.constraint_name
		WHERE tc.table_name = ? AND tc.constraint_type = 'FOREIGN KEY'
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var name, fromCol, toTable, toCol, onDelete, onUpdate string
		if err := rows.Scan(&name, &fromCol, &toTable, &toCol, &onDelete, &onUpdate); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}

		foreignKeys = append(foreignKeys, ForeignKey{
			Name:       name,
			FromColumn: fromCol,
			ToTable:    toTable,
			ToColumn:   toCol,
			OnDelete:   onDelete,
			OnUpdate:   onUpdate,
		})
	}

	return foreignKeys, nil
}

func (dc *DBConnection) readSQLiteSchema() (*Schema, error) {
	rows, err := dc.DB.Query(`
		SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		table, err := dc.readSQLiteTable(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to read table %s: %w", tableName, err)
		}
		tables = append(tables, table)
	}

	return &Schema{Tables: tables}, nil
}

func (dc *DBConnection) readSQLiteTable(tableName string) (Table, error) {
	var schema string
	err := dc.DB.QueryRow(fmt.Sprintf("SELECT sql FROM sqlite_master WHERE type='table' AND name='%s'", tableName)).Scan(&schema)
	if err != nil {
		return Table{}, fmt.Errorf("failed to get table schema: %w", err)
	}

	columns, err := dc.readSQLiteColumns(tableName)
	if err != nil {
		return Table{}, err
	}

	indexes, err := dc.readSQLiteIndexes(tableName)
	if err != nil {
		return Table{}, err
	}

	return Table{
		Name:        tableName,
		Columns:     columns,
		Indexes:     indexes,
		ForeignKeys: []ForeignKey{},
	}, nil
}

func (dc *DBConnection) readSQLiteColumns(tableName string) ([]Column, error) {
	rows, err := dc.DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		columns = append(columns, Column{
			Name:          name,
			Type:          colType,
			Nullable:      notNull == 0,
			DefaultValue:  defaultValue.String,
			PrimaryKey:    pk != 0,
			AutoIncrement: pk != 0 && strings.Contains(strings.ToLower(colType), "integer"),
		})
	}

	return columns, nil
}

func (dc *DBConnection) readSQLiteIndexes(tableName string) ([]Index, error) {
	rows, err := dc.DB.Query(fmt.Sprintf("PRAGMA index_list(%s)", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var seq, name string
		var unique int
		if err := rows.Scan(&seq, &name, &unique); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		idxRows, err := dc.DB.Query(fmt.Sprintf("PRAGMA index_info(%s)", name))
		if err != nil {
			return nil, fmt.Errorf("failed to get index info: %w", err)
		}

		var columns []string
		for idxRows.Next() {
			var seqNo, cid int
			var colName string
			if err := idxRows.Scan(&seqNo, &cid, &colName); err != nil {
				idxRows.Close()
				return nil, fmt.Errorf("failed to scan index column: %w", err)
			}
			columns = append(columns, colName)
		}
		idxRows.Close()

		indexes = append(indexes, Index{
			Name:    name,
			Unique:  unique != 0,
			Columns: columns,
		})
	}

	return indexes, nil
}
