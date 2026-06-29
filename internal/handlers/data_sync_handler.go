package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"dbsync/internal/audit"
	"dbsync/internal/auth"
	"dbsync/internal/dbcompare"
	"dbsync/internal/datasync"
)

type DataSyncHandler struct {
	db *sql.DB
}

func NewDataSyncHandler(db *sql.DB) *DataSyncHandler {
	return &DataSyncHandler{db: db}
}

type DataSyncRequest struct {
	SourceID     int      `json:"source_id"`
	TargetIDs    []int    `json:"target_ids"`
	Tables       []string `json:"tables"`
	DeleteMissing bool    `json:"delete_missing"`
}

type DataSyncResponse struct {
	TargetID   int                   `json:"target_id"`
	TargetName string                `json:"target_name"`
	Results    []*datasync.SyncResult `json:"results"`
	Error      string                `json:"error,omitempty"`
}

func (h *DataSyncHandler) SyncData(w http.ResponseWriter, r *http.Request) {
	var req DataSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	token := r.Header.Get("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	claims, err := auth.ValidateToken(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	sourceConfig, err := audit.GetDBConfigByID(h.db, req.SourceID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get source config"})
		return
	}
	if sourceConfig == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Source config not found"})
		return
	}

	sourceDB := &dbcompare.DBConnection{
		DBType:   sourceConfig.DBType,
		Host:     sourceConfig.Host,
		Port:     sourceConfig.Port,
		Database: sourceConfig.Database,
		Username: sourceConfig.Username,
		Password: sourceConfig.Password,
	}
	if err := sourceDB.Connect(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to connect to source DB: " + err.Error()})
		return
	}
	defer sourceDB.Close()

	var results []DataSyncResponse
	for _, targetID := range req.TargetIDs {
		targetConfig, err := audit.GetDBConfigByID(h.db, targetID)
		if err != nil {
			results = append(results, DataSyncResponse{
				TargetID: targetID,
				Error:    "Failed to get target config",
			})
			continue
		}
		if targetConfig == nil {
			results = append(results, DataSyncResponse{
				TargetID: targetID,
				Error:    "Target config not found",
			})
			continue
		}

		targetDB := &dbcompare.DBConnection{
			DBType:   targetConfig.DBType,
			Host:     targetConfig.Host,
			Port:     targetConfig.Port,
			Database: targetConfig.Database,
			Username: targetConfig.Username,
			Password: targetConfig.Password,
		}
		if err := targetDB.Connect(); err != nil {
			results = append(results, DataSyncResponse{
				TargetID:   targetID,
				TargetName: targetConfig.Name,
				Error:      "Failed to connect to target DB: " + err.Error(),
			})
			continue
		}

		var tableResults []*datasync.SyncResult
		if len(req.Tables) > 0 {
			for _, table := range req.Tables {
				result, err := datasync.SyncTableData(sourceDB, targetDB, table, req.DeleteMissing)
				if err != nil {
					tableResults = append(tableResults, &datasync.SyncResult{
						Table: table,
						Error: err.Error(),
					})
				} else {
					tableResults = append(tableResults, result)
				}
			}
		} else {
			allResults, err := datasync.SyncAllTables(sourceDB, targetDB, req.DeleteMissing)
			if err != nil {
				targetDB.Close()
				results = append(results, DataSyncResponse{
					TargetID:   targetID,
					TargetName: targetConfig.Name,
					Error:      "Failed to sync all tables: " + err.Error(),
				})
				continue
			}
			tableResults = allResults
		}

		targetDB.Close()

		results = append(results, DataSyncResponse{
			TargetID:   targetID,
			TargetName: targetConfig.Name,
			Results:    tableResults,
		})

		var success bool
		for _, r := range tableResults {
			if r.Error != "" {
				success = false
				break
			}
		}
		success = len(tableResults) > 0 && success

		status := "SUCCESS"
		if !success {
			status = "PARTIAL"
		}
		audit.LogAction(h.db, claims.UserID, "SYNC_DATA", "data", "", sourceConfig.Name, targetConfig.Name, "", status)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *DataSyncHandler) GetTablePreview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	configID, err := strconv.Atoi(vars["config_id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid config ID"})
		return
	}

	tableName := vars["table_name"]

	config, err := audit.GetDBConfigByID(h.db, configID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get config"})
		return
	}

	db := &dbcompare.DBConnection{
		DBType:   config.DBType,
		Host:     config.Host,
		Port:     config.Port,
		Database: config.Database,
		Username: config.Username,
		Password: config.Password,
	}
	if err := db.Connect(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to connect: " + err.Error()})
		return
	}
	defer db.Close()

	rows, err := datasync.GetTableRowSample(db, tableName, 10)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get table data: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rows)
}

func (h *DataSyncHandler) GetTableCount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	configID, err := strconv.Atoi(vars["config_id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid config ID"})
		return
	}

	tableName := vars["table_name"]

	config, err := audit.GetDBConfigByID(h.db, configID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get config"})
		return
	}

	db := &dbcompare.DBConnection{
		DBType:   config.DBType,
		Host:     config.Host,
		Port:     config.Port,
		Database: config.Database,
		Username: config.Username,
		Password: config.Password,
	}
	if err := db.Connect(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to connect: " + err.Error()})
		return
	}
	defer db.Close()

	count, err := datasync.CountTableRows(db, tableName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to count rows: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

func (h *DataSyncHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var config audit.DBConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	err := datasync.TestConnection(config.DBType, config.Host, config.Port, config.Database, config.Username, config.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
