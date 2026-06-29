package dbcompare

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

type DBConnection struct {
	DBType   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	DB       *sql.DB
}

func (dc *DBConnection) Connect() error {
	var dsn string
	var driverName string
	switch dc.DBType {
	case "mysql":
		driverName = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
	case "postgres":
		driverName = "pgx"
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			dc.Host, dc.Port, dc.Username, dc.Password, dc.Database)
	case "sqlite":
		driverName = "sqlite3"
		dsn = dc.Database
	default:
		return fmt.Errorf("unsupported database type: %s", dc.DBType)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	dc.DB = db
	return nil
}

func (dc *DBConnection) Close() {
	if dc.DB != nil {
		dc.DB.Close()
	}
}
