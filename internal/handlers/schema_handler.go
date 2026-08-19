package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"dbsync/internal/audit"
	"dbsync/internal/auth"
	"dbsync/internal/dbcompare"
)

type SchemaHandler struct {
	db          *sql.DB
	cacheMu     sync.RWMutex
	tablesCache map[int]schemaTablesCacheEntry
}

const schemaTablesCacheTTL = 5 * time.Minute

type schemaTablesCacheEntry struct {
	tables    []string
	expiresAt time.Time
}

func NewSchemaHandler(db *sql.DB) *SchemaHandler {
	return &SchemaHandler{db: db, tablesCache: make(map[int]schemaTablesCacheEntry)}
}

type SchemaCompareRequest struct {
	SourceID  int      `json:"source_id"`
	TargetIDs []int    `json:"target_ids"`
	Tables    []string `json:"tables,omitempty"`
}

type SchemaCompareResponse struct {
	TargetID   int                   `json:"target_id"`
	TargetName string                `json:"target_name"`
	Diff       *dbcompare.SchemaDiff `json:"diff"`
	SQL        []string              `json:"sql"`
	Error      string                `json:"error,omitempty"`
}

type SchemaCompareLog struct {
	Level            string   `json:"level"`
	TargetName       string   `json:"target_name,omitempty"`
	TableName        string   `json:"table_name,omitempty"`
	ColumnName       string   `json:"column_name,omitempty"`
	Progress         float64  `json:"progress,omitempty"`
	ElapsedSeconds   int64    `json:"elapsed_seconds,omitempty"`
	RemainingSeconds int64    `json:"remaining_seconds,omitempty"`
	Message          string   `json:"message"`
	SQL              []string `json:"sql,omitempty"`
}

const (
	schemaTargetWorkerMultiplier = 2
	schemaTableWorkerLimit       = 4
	schemaLogLevelError          = "error"
	schemaLogLevelInfo           = "info"
)

type schemaProgressTracker struct {
	startTime time.Time
	total     int
	current   int
	mutex     sync.Mutex
}

func (h *SchemaHandler) CompareSchemas(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSchemaCompareRequest(r)
	if err != nil {
		log.Printf("[SchemaCompare] ERROR: Invalid request - %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	results, err := h.compareSchemas(req, nil)
	if err != nil {
		log.Printf("[SchemaCompare] ERROR: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *SchemaHandler) CompareSchemasStream(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSchemaCompareRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Streaming is not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var writeMutex sync.Mutex
	emitEvent := func(event string, data interface{}) {
		writeMutex.Lock()
		defer writeMutex.Unlock()
		writeSchemaCompareSSE(w, flusher, event, data)
	}

	emitEvent("log", SchemaCompareLog{
		Level:   schemaLogLevelInfo,
		Message: "已开始表结构比对",
	})

	results, err := h.compareSchemas(req, func(progress SchemaCompareLog) {
		emitEvent("log", progress)
	})
	if err != nil {
		log.Printf("[SchemaCompare] ERROR: %v", err)
		emitEvent("error", SchemaCompareLog{
			Level:   schemaLogLevelError,
			Message: err.Error(),
		})
		return
	}

	emitEvent("complete", map[string]interface{}{"completed": true, "results": results})
	return
}

func (h *SchemaHandler) compareSchemas(req SchemaCompareRequest, progressFunc func(SchemaCompareLog)) ([]SchemaCompareResponse, error) {
	notifySchemaCompareLog(progressFunc, SchemaCompareLog{
		Level:   schemaLogLevelInfo,
		Message: "正在读取源数据库配置",
	})
	sourceConfig, err := audit.GetDBConfigByID(h.db, req.SourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source config: %w", err)
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not found")
	}

	log.Printf("[SchemaCompare] Connecting to source DB: %s (%s:%d/%s)", sourceConfig.Name, sourceConfig.Host, sourceConfig.Port, sourceConfig.Database)
	notifySchemaCompareLog(progressFunc, SchemaCompareLog{
		Level:   schemaLogLevelInfo,
		Message: "正在连接源数据库",
	})
	sourceDB := &dbcompare.DBConnection{
		DBType:   sourceConfig.DBType,
		Host:     sourceConfig.Host,
		Port:     sourceConfig.Port,
		Database: sourceConfig.Database,
		Username: sourceConfig.Username,
		Password: sourceConfig.Password,
	}
	if err := sourceDB.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to source DB: %w", err)
	}
	defer sourceDB.Close()

	log.Printf("[SchemaCompare] Reading source schema")
	notifySchemaCompareLog(progressFunc, SchemaCompareLog{
		Level:   schemaLogLevelInfo,
		Message: "正在读取源数据库表结构",
	})
	sourceSchema, err := sourceDB.ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read source schema: %w", err)
	}
	if len(req.Tables) > 0 {
		var found int
		sourceSchema, found = filterSchemaTables(sourceSchema, req.Tables)
		if found != len(req.Tables) {
			return nil, fmt.Errorf("one or more selected tables were not found in source database")
		}
	}
	log.Printf("[SchemaCompare] Source schema read: %d tables", len(sourceSchema.Tables))
	notifySchemaCompareLog(progressFunc, SchemaCompareLog{
		Level:   schemaLogLevelInfo,
		Message: fmt.Sprintf("源数据库读取完成，共 %d 张表", len(sourceSchema.Tables)),
	})
	progressTracker := newSchemaProgressTracker(len(req.TargetIDs) * countSchemaCompareUnits(sourceSchema))

	concurrency := runtime.NumCPU() * schemaTargetWorkerMultiplier
	log.Printf("[SchemaCompare] Using concurrency: %d (CPU cores: %d)", concurrency, runtime.NumCPU())

	targetConfigs := make([]*audit.DBConfig, 0, len(req.TargetIDs))
	for _, targetID := range req.TargetIDs {
		targetConfig, err := audit.GetDBConfigByID(h.db, targetID)
		if err != nil {
			log.Printf("[SchemaCompare] ERROR: Failed to get target config for id=%d - %v", targetID, err)
			targetConfigs = append(targetConfigs, nil)
			continue
		}
		if targetConfig == nil {
			log.Printf("[SchemaCompare] ERROR: Target config not found for id=%d", targetID)
			targetConfigs = append(targetConfigs, nil)
			continue
		}
		targetConfigs = append(targetConfigs, targetConfig)
	}

	type compareTask struct {
		targetConfig *audit.DBConfig
		targetID     int
	}

	tasks := make(chan compareTask, len(targetConfigs))
	for i, tc := range targetConfigs {
		tasks <- compareTask{targetConfig: tc, targetID: req.TargetIDs[i]}
	}
	close(tasks)

	resultsChan := make(chan SchemaCompareResponse, len(targetConfigs))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Printf("[SchemaCompare] Worker %d started", workerID)
			for task := range tasks {
				if task.targetConfig == nil {
					log.Printf("[SchemaCompare] Worker %d: Skipping nil target config for id=%d", workerID, task.targetID)
					resultsChan <- SchemaCompareResponse{
						TargetID: task.targetID,
						Error:    "Target config not found",
					}
					continue
				}

				notifySchemaCompareLog(progressFunc, SchemaCompareLog{
					Level:      schemaLogLevelInfo,
					TargetName: task.targetConfig.Name,
					Message:    "正在连接目标数据库",
				})
				log.Printf("[SchemaCompare] Worker %d: Processing target %s (%s:%d/%s)", workerID, task.targetConfig.Name, task.targetConfig.Host, task.targetConfig.Port, task.targetConfig.Database)
				targetDB := &dbcompare.DBConnection{
					DBType:   task.targetConfig.DBType,
					Host:     task.targetConfig.Host,
					Port:     task.targetConfig.Port,
					Database: task.targetConfig.Database,
					Username: task.targetConfig.Username,
					Password: task.targetConfig.Password,
				}
				if err := targetDB.Connect(); err != nil {
					log.Printf("[SchemaCompare] Worker %d ERROR: Failed to connect to target DB %s - %v", workerID, task.targetConfig.Name, err)
					resultsChan <- SchemaCompareResponse{
						TargetID:   task.targetID,
						TargetName: task.targetConfig.Name,
						Error:      "Failed to connect to target DB: " + err.Error(),
					}
					continue
				}

				log.Printf("[SchemaCompare] Worker %d: Reading target schema for %s", workerID, task.targetConfig.Name)
				notifySchemaCompareLog(progressFunc, SchemaCompareLog{
					Level:      schemaLogLevelInfo,
					TargetName: task.targetConfig.Name,
					Message:    "正在读取目标数据库表结构",
				})
				targetSchema, err := targetDB.ReadSchema()
				if err != nil {
					log.Printf("[SchemaCompare] Worker %d ERROR: Failed to read target schema for %s - %v", workerID, task.targetConfig.Name, err)
					targetDB.Close()
					resultsChan <- SchemaCompareResponse{
						TargetID:   task.targetID,
						TargetName: task.targetConfig.Name,
						Error:      "Failed to read target schema: " + err.Error(),
					}
					continue
				}
				if len(req.Tables) > 0 {
					targetSchema, _ = filterSchemaTables(targetSchema, req.Tables)
				}
				log.Printf("[SchemaCompare] Worker %d: Target schema read for %s: %d tables", workerID, task.targetConfig.Name, len(targetSchema.Tables))
				notifySchemaCompareLog(progressFunc, SchemaCompareLog{
					Level:      schemaLogLevelInfo,
					TargetName: task.targetConfig.Name,
					Message:    fmt.Sprintf("目标数据库读取完成，共 %d 张表", len(targetSchema.Tables)),
				})

				log.Printf("[SchemaCompare] Worker %d: Comparing schemas: %s -> %s", workerID, sourceConfig.Name, task.targetConfig.Name)
				notifySchemaCompareLog(progressFunc, SchemaCompareLog{
					Level:      schemaLogLevelInfo,
					TargetName: task.targetConfig.Name,
					Message:    "开始并发比对表和字段",
				})
				diff := dbcompare.CompareSchemasWithProgress(sourceSchema, targetSchema, schemaTableWorkerLimit, func(progress dbcompare.SchemaCompareProgress) {
					progressLog := SchemaCompareLog{
						Level:      schemaLogLevelInfo,
						TargetName: task.targetConfig.Name,
						TableName:  progress.TableName,
						ColumnName: progress.ColumnName,
						Message:    progress.Message,
					}
					if progress.TableDiff != nil {
						progressLog.SQL = dbcompare.GenerateSQL(&dbcompare.SchemaDiff{TableDiffs: []dbcompare.TableDiff{*progress.TableDiff}}, task.targetConfig.DBType)
					}
					notifySchemaCompareLog(progressFunc, progressTracker.next(progressLog))
				})
				sqlStatements := dbcompare.GenerateSQL(diff, task.targetConfig.DBType)
				targetDB.Close()

				diffCount := 0
				for _, td := range diff.TableDiffs {
					diffCount += len(td.ColumnDiffs) + 1
				}
				log.Printf("[SchemaCompare] Worker %d: Schema compared: %d differences found, %d SQL statements generated", workerID, diffCount, len(sqlStatements))
				notifySchemaCompareLog(progressFunc, progressTracker.complete(SchemaCompareLog{
					Level:      schemaLogLevelInfo,
					TargetName: task.targetConfig.Name,
					Message:    fmt.Sprintf("目标数据库比对完成，发现 %d 处差异，生成 %d 条SQL", diffCount, len(sqlStatements)),
				}))

				resultsChan <- SchemaCompareResponse{
					TargetID:   task.targetID,
					TargetName: task.targetConfig.Name,
					Diff:       diff,
					SQL:        sqlStatements,
				}
			}
			log.Printf("[SchemaCompare] Worker %d finished", workerID)
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []SchemaCompareResponse
	for result := range resultsChan {
		results = append(results, result)
	}

	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].TargetID < results[i].TargetID {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	log.Printf("[SchemaCompare] Completed: %d targets processed", len(results))
	return results, nil
}

func newSchemaProgressTracker(total int) *schemaProgressTracker {
	if total < 1 {
		total = 1
	}
	return &schemaProgressTracker{
		startTime: time.Now(),
		total:     total,
	}
}

func (tracker *schemaProgressTracker) next(log SchemaCompareLog) SchemaCompareLog {
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()

	tracker.current++
	if tracker.current > tracker.total {
		tracker.current = tracker.total
	}
	elapsedSeconds := int64(time.Since(tracker.startTime).Seconds())
	progress := float64(tracker.current) / float64(tracker.total) * 100
	if progress > 99 {
		progress = 99
	}
	log.Progress = progress
	log.ElapsedSeconds = elapsedSeconds
	if tracker.current > 0 && tracker.current < tracker.total {
		estimatedTotalSeconds := float64(elapsedSeconds) / float64(tracker.current) * float64(tracker.total)
		log.RemainingSeconds = int64(estimatedTotalSeconds) - elapsedSeconds
	}
	return log
}

func (tracker *schemaProgressTracker) complete(log SchemaCompareLog) SchemaCompareLog {
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()

	elapsedSeconds := int64(time.Since(tracker.startTime).Seconds())
	log.Progress = 100
	log.ElapsedSeconds = elapsedSeconds
	log.RemainingSeconds = 0
	return log
}

func countSchemaCompareUnits(schema *dbcompare.Schema) int {
	total := 0
	for _, table := range schema.Tables {
		total++
		total += len(table.Columns)
	}
	if total < 1 {
		return 1
	}
	return total
}

func decodeSchemaCompareRequest(r *http.Request) (SchemaCompareRequest, error) {
	var req SchemaCompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return SchemaCompareRequest{}, err
	}
	if req.SourceID <= 0 || len(req.TargetIDs) == 0 {
		return SchemaCompareRequest{}, fmt.Errorf("source database and target database are required")
	}
	return req, nil
}

// filterSchemaTables 保持数据库原始顺序，只保留用户选择的表，并返回实际命中的表数量。
func filterSchemaTables(schema *dbcompare.Schema, selected []string) (*dbcompare.Schema, int) {
	selectedSet := make(map[string]struct{}, len(selected))
	for _, tableName := range selected {
		selectedSet[tableName] = struct{}{}
	}
	filtered := make([]dbcompare.Table, 0, len(selectedSet))
	for _, table := range schema.Tables {
		if _, ok := selectedSet[table.Name]; ok {
			filtered = append(filtered, table)
		}
	}
	return &dbcompare.Schema{Tables: filtered}, len(filtered)
}

func notifySchemaCompareLog(progressFunc func(SchemaCompareLog), progress SchemaCompareLog) {
	if progressFunc != nil {
		progressFunc(progress)
	}
}

func writeSchemaCompareSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("[SchemaCompare] ERROR: Failed to marshal SSE payload - %v", err)
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
}

type SchemaExecuteRequest struct {
	SourceID int      `json:"source_id"`
	TargetID int      `json:"target_id"`
	SQL      []string `json:"sql"`
}

func (h *SchemaHandler) ExecuteSchemaSQL(w http.ResponseWriter, r *http.Request) {
	var req SchemaExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[SchemaExecute] ERROR: Invalid request - %v", err)
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
		log.Printf("[SchemaExecute] ERROR: Unauthorized - %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	sourceConfig, err := audit.GetDBConfigByID(h.db, req.SourceID)
	if err != nil {
		log.Printf("[SchemaExecute] ERROR: Failed to get source config - %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get source config"})
		return
	}

	targetConfig, err := audit.GetDBConfigByID(h.db, req.TargetID)
	if err != nil {
		log.Printf("[SchemaExecute] ERROR: Failed to get target config - %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get target config"})
		return
	}

	sqlContent := strings.Join(req.SQL, "\n")
	log.Printf("[SchemaExecute] Start: source=%s, target=%s, sql_count=%d", sourceConfig.Name, targetConfig.Name, len(req.SQL))

	targetDB := &dbcompare.DBConnection{
		DBType:   targetConfig.DBType,
		Host:     targetConfig.Host,
		Port:     targetConfig.Port,
		Database: targetConfig.Database,
		Username: targetConfig.Username,
		Password: targetConfig.Password,
	}
	if err := targetDB.Connect(); err != nil {
		log.Printf("[SchemaExecute] ERROR: Failed to connect to target DB %s - %v", targetConfig.Name, err)
		audit.LogAction(h.db, claims.UserID, "EXECUTE_SCHEMA_SQL", "schema", "", sourceConfig.Name, targetConfig.Name, "Failed to connect: "+err.Error(), sqlContent, "FAILED")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to connect to target DB: " + err.Error()})
		return
	}
	defer targetDB.Close()

	executedCount := 0
	var errors []string
	for i, sqlStmt := range req.SQL {
		sqlStmt = strings.TrimSpace(sqlStmt)
		if sqlStmt == "" || strings.HasPrefix(sqlStmt, "--") {
			continue
		}
		log.Printf("[SchemaExecute] Executing SQL %d/%d: %s", i+1, len(req.SQL), truncateSQL(sqlStmt))
		if _, err := targetDB.DB.Exec(sqlStmt); err != nil {
			errMsg := fmt.Sprintf("SQL %d failed: %v", i+1, err)
			log.Printf("[SchemaExecute] ERROR: %s", errMsg)
			errors = append(errors, errMsg)
		} else {
			executedCount++
			log.Printf("[SchemaExecute] SQL %d executed successfully", i+1)
		}
	}

	status := "SUCCESS"
	details := fmt.Sprintf("Executed %d/%d SQL statements", executedCount, len(req.SQL))
	if len(errors) > 0 {
		status = "PARTIAL"
		details += "; Errors: " + strings.Join(errors, "; ")
	}

	log.Printf("[SchemaExecute] Completed: %s, %s", status, details)
	audit.LogAction(h.db, claims.UserID, "EXECUTE_SCHEMA_SQL", "schema", "", sourceConfig.Name, targetConfig.Name, details, sqlContent, status)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "SQL execution completed",
		"executed": executedCount,
		"total":    len(req.SQL),
		"errors":   errors,
		"status":   status,
	})
}

func truncateSQL(sql string) string {
	if len(sql) <= 100 {
		return sql
	}
	return sql[:100] + "..."
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
	if tables, ok := h.cachedTables(configID); ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tables)
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
	h.cacheMu.Lock()
	h.tablesCache[configID] = schemaTablesCacheEntry{tables: append([]string(nil), tables...), expiresAt: time.Now().Add(schemaTablesCacheTTL)}
	h.cacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tables)
}

func (h *SchemaHandler) cachedTables(configID int) ([]string, bool) {
	h.cacheMu.RLock()
	entry, ok := h.tablesCache[configID]
	h.cacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return append([]string(nil), entry.tables...), true
}
