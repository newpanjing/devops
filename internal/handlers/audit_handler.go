package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"dbsync/internal/audit"
	"dbsync/internal/auth"
	"dbsync/internal/dbcompare"
)

type AuditHandler struct {
	db *sql.DB
}

func NewAuditHandler(db *sql.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

func (h *AuditHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	logs, err := audit.GetLogs(h.db, limit, offset)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(logs)
}

func (h *AuditHandler) ApplySQL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid log ID"})
		return
	}

	token := r.Header.Get("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	_, err = auth.ValidateToken(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	logEntry, err := audit.GetLogByID(h.db, logID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get log"})
		return
	}
	if logEntry == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Log not found"})
		return
	}
	if logEntry.SQL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "No SQL to apply"})
		return
	}

	targetDBName := logEntry.TargetDB
	if targetDBName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Target database not specified"})
		return
	}

	targetConfig, err := getDBConfigByName(h.db, targetDBName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get target config: " + err.Error()})
		return
	}
	if targetConfig == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Target config not found"})
		return
	}

	db := &dbcompare.DBConnection{
		DBType:   targetConfig.DBType,
		Host:     targetConfig.Host,
		Port:     targetConfig.Port,
		Database: targetConfig.Database,
		Username: targetConfig.Username,
		Password: targetConfig.Password,
	}
	if err := db.Connect(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to connect: " + err.Error()})
		return
	}
	defer db.Close()

	sqlStatements := strings.Split(logEntry.SQL, ";")
	for _, sqlStmt := range sqlStatements {
		sqlStmt = strings.TrimSpace(sqlStmt)
		if sqlStmt == "" || strings.HasPrefix(sqlStmt, "--") {
			continue
		}
		if _, err := db.DB.Exec(sqlStmt + ";"); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to execute SQL: %s", err.Error())})
			return
		}
	}

	if err := audit.UpdateLogApplied(h.db, logID, 1); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update log status"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "SQL applied successfully"})
}

func getDBConfigByName(db *sql.DB, name string) (*audit.DBConfig, error) {
	var config audit.DBConfig
	err := db.QueryRow(`
		SELECT id, name, db_type, host, port, database, username, password
		FROM db_configs
		WHERE name = ?
	`, name).Scan(
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
		return nil, err
	}
	return &config, nil
}
