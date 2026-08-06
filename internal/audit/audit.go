package audit

import (
	"database/sql"
	"fmt"
	"time"
)

type AuditLog struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetName string    `json:"target_name"`
	SourceDB   string    `json:"source_db"`
	TargetDB   string    `json:"target_db"`
	Details    string    `json:"details"`
	SQL        string    `json:"sql"`
	Applied    int       `json:"applied"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type DBConfig struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	DBType   string `json:"db_type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func LogAction(db *sql.DB, userID int, action, targetType, targetName, sourceDB, targetDB, details, sql, status string) error {
	_, err := db.Exec(`
		INSERT INTO audit_logs (user_id, action, target_type, target_name, source_db, target_db, details, sql, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, action, targetType, targetName, sourceDB, targetDB, details, sql, status)
	if err != nil {
		return fmt.Errorf("failed to log action: %w", err)
	}
	return nil
}

func GetLogs(db *sql.DB, limit, offset int) ([]AuditLog, error) {
	rows, err := db.Query(`
		SELECT id, user_id, action, target_type, target_name, source_db, target_db, details, sql, applied, status, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		if err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Action,
			&log.TargetType,
			&log.TargetName,
			&log.SourceDB,
			&log.TargetDB,
			&log.Details,
			&log.SQL,
			&log.Applied,
			&log.Status,
			&log.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func CreateDBConfig(db *sql.DB, config *DBConfig) error {
	_, err := db.Exec(`
		INSERT INTO db_configs (name, db_type, host, port, database, username, password)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, config.Name, config.DBType, config.Host, config.Port, config.Database, config.Username, config.Password)
	if err != nil {
		return fmt.Errorf("failed to create db config: %w", err)
	}
	return nil
}

func GetDBConfigs(db *sql.DB) ([]DBConfig, error) {
	rows, err := db.Query(`
		SELECT id, name, db_type, host, port, database, username, password
		FROM db_configs
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query db configs: %w", err)
	}
	defer rows.Close()

	configs := []DBConfig{}
	for rows.Next() {
		var config DBConfig
		if err := rows.Scan(
			&config.ID,
			&config.Name,
			&config.DBType,
			&config.Host,
			&config.Port,
			&config.Database,
			&config.Username,
			&config.Password,
		); err != nil {
			return nil, fmt.Errorf("failed to scan db config: %w", err)
		}
		configs = append(configs, config)
	}

	return configs, nil
}

func GetDBConfigByID(db *sql.DB, id int) (*DBConfig, error) {
	var config DBConfig
	err := db.QueryRow(`
		SELECT id, name, db_type, host, port, database, username, password
		FROM db_configs
		WHERE id = ?
	`, id).Scan(
		&config.ID,
		&config.Name,
		&config.DBType,
		&config.Host,
		&config.Port,
		&config.Database,
		&config.Username,
		&config.Password,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get db config: %w", err)
	}
	return &config, nil
}

func DeleteDBConfig(db *sql.DB, id int) error {
	_, err := db.Exec(`DELETE FROM db_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete db config: %w", err)
	}
	return nil
}

func GetLogByID(db *sql.DB, id int) (*AuditLog, error) {
	var log AuditLog
	err := db.QueryRow(`
		SELECT id, user_id, action, target_type, target_name, source_db, target_db, details, sql, applied, status, created_at
		FROM audit_logs
		WHERE id = ?
	`, id).Scan(
		&log.ID,
		&log.UserID,
		&log.Action,
		&log.TargetType,
		&log.TargetName,
		&log.SourceDB,
		&log.TargetDB,
		&log.Details,
		&log.SQL,
		&log.Applied,
		&log.Status,
		&log.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get log: %w", err)
	}
	return &log, nil
}

func UpdateLogApplied(db *sql.DB, id int, applied int) error {
	_, err := db.Exec(`UPDATE audit_logs SET applied = ? WHERE id = ?`, applied, id)
	if err != nil {
		return fmt.Errorf("failed to update log: %w", err)
	}
	return nil
}
