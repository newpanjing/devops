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
)

type SchemaHandler struct {
	db *sql.DB
}

func NewSchemaHandler(db *sql.DB) *SchemaHandler {
	return &SchemaHandler{db: db}
}

type SchemaCompareRequest struct {
	SourceID int   `json:"source_id"`
	TargetIDs []int `json:"target_ids"`
}

type SchemaCompareResponse struct {
	TargetID int              `json:"target_id"`
	TargetName string         `json:"target_name"`
	Diff     *dbcompare.SchemaDiff `json:"diff"`
	Error    string           `json:"error,omitempty"`
}

func (h *SchemaHandler) CompareSchemas(w http.ResponseWriter, r *http.Request) {
	var req SchemaCompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
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

	sourceSchema, err := sourceDB.ReadSchema()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read source schema: " + err.Error()})
		return
	}

	var results []SchemaCompareResponse
	for _, targetID := range req.TargetIDs {
		targetConfig, err := audit.GetDBConfigByID(h.db, targetID)
		if err != nil {
			results = append(results, SchemaCompareResponse{
				TargetID: targetID,
				Error:    "Failed to get target config",
			})
			continue
		}
		if targetConfig == nil {
			results = append(results, SchemaCompareResponse{
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
			results = append(results, SchemaCompareResponse{
				TargetID: targetID,
				TargetName: targetConfig.Name,
				Error:    "Failed to connect to target DB: " + err.Error(),
			})
			continue
		}

		targetSchema, err := targetDB.ReadSchema()
		if err != nil {
			targetDB.Close()
			results = append(results, SchemaCompareResponse{
				TargetID: targetID,
				TargetName: targetConfig.Name,
				Error:    "Failed to read target schema: " + err.Error(),
			})
			continue
		}

		diff := dbcompare.CompareSchemas(sourceSchema, targetSchema)
		targetDB.Close()

		results = append(results, SchemaCompareResponse{
			TargetID: targetID,
			TargetName: targetConfig.Name,
			Diff:     diff,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

type SchemaSyncRequest struct {
	SourceID int   `json:"source_id"`
	TargetID int   `json:"target_id"`
	Diff     *dbcompare.SchemaDiff `json:"diff"`
}

func (h *SchemaHandler) SyncSchema(w http.ResponseWriter, r *http.Request) {
	var req SchemaSyncRequest
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

	targetConfig, err := audit.GetDBConfigByID(h.db, req.TargetID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get target config"})
		return
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to connect to target DB: " + err.Error()})
		return
	}
	defer targetDB.Close()

	if err := targetDB.SyncSchema(req.Diff); err != nil {
		audit.LogAction(h.db, claims.UserID, "SYNC_SCHEMA", "schema", "", sourceConfig.Name, targetConfig.Name, err.Error(), "FAILED")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to sync schema: " + err.Error()})
		return
	}

	audit.LogAction(h.db, claims.UserID, "SYNC_SCHEMA", "schema", "", sourceConfig.Name, targetConfig.Name, "Schema synced successfully", "SUCCESS")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Schema synced successfully"})
}

func (h *SchemaHandler) GetTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	configID, err := strconv.Atoi(vars["config_id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid config ID"})
		return
	}

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

	schema, err := db.ReadSchema()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read schema: " + err.Error()})
		return
	}

	var tables []string
	for _, table := range schema.Tables {
		tables = append(tables, table.Name)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tables)
}
