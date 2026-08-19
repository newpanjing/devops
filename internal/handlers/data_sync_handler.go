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
	"dbsync/internal/datasync"
	"dbsync/internal/dbcompare"
)

type DataSyncHandler struct {
	db           *sql.DB
	cacheMu      sync.RWMutex
	columnsCache map[string]dataSyncColumnsCacheEntry
}

const dataSyncColumnsCacheTTL = 5 * time.Minute

type dataSyncColumnsCacheEntry struct {
	columns   []string
	expiresAt time.Time
}

func NewDataSyncHandler(db *sql.DB) *DataSyncHandler {
	return &DataSyncHandler{db: db, columnsCache: make(map[string]dataSyncColumnsCacheEntry)}
}

type DataSyncRequest struct {
	SourceID      int               `json:"source_id"`
	TargetIDs     []int             `json:"target_ids"`
	Tables        []string          `json:"tables"`
	KeyFields     map[string]string `json:"key_fields"`
	DeleteMissing bool              `json:"delete_missing"`
}

type DataSyncResponse struct {
	TargetID   int                    `json:"target_id"`
	TargetName string                 `json:"target_name"`
	Results    []*datasync.SyncResult `json:"results"`
	Error      string                 `json:"error,omitempty"`
}

type DataSyncLog struct {
	Level      string `json:"level"`
	TargetName string `json:"target_name,omitempty"`
	TableName  string `json:"table_name,omitempty"`
	Message    string `json:"message"`
}

// testConnectionRequest 同时支持已保存配置 ID 和新增配置表单的完整连接参数。
type testConnectionRequest struct {
	ConfigID int `json:"config_id"`
	audit.DBConfig
}

const (
	dataSyncLogLevelError          = "error"
	dataSyncLogLevelInfo           = "info"
	dataSyncTargetWorkerMultiplier = 2
)

func (h *DataSyncHandler) SyncData(w http.ResponseWriter, r *http.Request) {
	var req DataSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[DataSync] ERROR: Invalid request - %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	log.Printf("[DataSync] Start preview: source_id=%d, target_ids=%v, tables=%v, delete_missing=%v", req.SourceID, req.TargetIDs, req.Tables, req.DeleteMissing)
	if err := validateDataSyncRequest(req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	sourceConfig, err := audit.GetDBConfigByID(h.db, req.SourceID)
	if err != nil {
		log.Printf("[DataSync] ERROR: Failed to get source config - %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get source config"})
		return
	}
	if sourceConfig == nil {
		log.Printf("[DataSync] ERROR: Source config not found for id=%d", req.SourceID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Source config not found"})
		return
	}

	log.Printf("[DataSync] Connecting to source DB: %s (%s:%d/%s)", sourceConfig.Name, sourceConfig.Host, sourceConfig.Port, sourceConfig.Database)
	sourceDB := &dbcompare.DBConnection{
		DBType:   sourceConfig.DBType,
		Host:     sourceConfig.Host,
		Port:     sourceConfig.Port,
		Database: sourceConfig.Database,
		Username: sourceConfig.Username,
		Password: sourceConfig.Password,
	}
	if err := sourceDB.Connect(); err != nil {
		log.Printf("[DataSync] ERROR: Failed to connect to source DB - %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to connect to source DB: " + err.Error()})
		return
	}
	defer sourceDB.Close()

	concurrency := runtime.NumCPU() * 2
	log.Printf("[DataSync] Using concurrency: %d (CPU cores: %d)", concurrency, runtime.NumCPU())

	targetConfigs := make([]*audit.DBConfig, 0, len(req.TargetIDs))
	for _, targetID := range req.TargetIDs {
		targetConfig, err := audit.GetDBConfigByID(h.db, targetID)
		if err != nil {
			log.Printf("[DataSync] ERROR: Failed to get target config for id=%d - %v", targetID, err)
			targetConfigs = append(targetConfigs, nil)
			continue
		}
		if targetConfig == nil {
			log.Printf("[DataSync] ERROR: Target config not found for id=%d", targetID)
			targetConfigs = append(targetConfigs, nil)
			continue
		}
		targetConfigs = append(targetConfigs, targetConfig)
	}

	type syncTask struct {
		targetConfig *audit.DBConfig
		targetID     int
	}

	tasks := make(chan syncTask, len(targetConfigs))
	for i, tc := range targetConfigs {
		tasks <- syncTask{targetConfig: tc, targetID: req.TargetIDs[i]}
	}
	close(tasks)

	resultsChan := make(chan DataSyncResponse, len(targetConfigs))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Printf("[DataSync] Worker %d started", workerID)
			for task := range tasks {
				if task.targetConfig == nil {
					log.Printf("[DataSync] Worker %d: Skipping nil target config for id=%d", workerID, task.targetID)
					resultsChan <- DataSyncResponse{
						TargetID: task.targetID,
						Error:    "Target config not found",
					}
					continue
				}

				log.Printf("[DataSync] Worker %d: Processing target %s (%s:%d/%s)", workerID, task.targetConfig.Name, task.targetConfig.Host, task.targetConfig.Port, task.targetConfig.Database)
				targetDB := &dbcompare.DBConnection{
					DBType:   task.targetConfig.DBType,
					Host:     task.targetConfig.Host,
					Port:     task.targetConfig.Port,
					Database: task.targetConfig.Database,
					Username: task.targetConfig.Username,
					Password: task.targetConfig.Password,
				}
				if err := targetDB.Connect(); err != nil {
					log.Printf("[DataSync] Worker %d ERROR: Failed to connect to target DB %s - %v", workerID, task.targetConfig.Name, err)
					resultsChan <- DataSyncResponse{
						TargetID:   task.targetID,
						TargetName: task.targetConfig.Name,
						Error:      "Failed to connect to target DB: " + err.Error(),
					}
					continue
				}

				var tableResults []*datasync.SyncResult
				for _, table := range req.Tables {
					log.Printf("[DataSync] Worker %d: Previewing table: %s", workerID, table)
					result, err := datasync.PreviewTableData(sourceDB, targetDB, table, req.KeyFields[table], req.DeleteMissing)
					if err != nil {
						log.Printf("[DataSync] Worker %d ERROR: Failed to preview table %s - %v", workerID, table, err)
						tableResults = append(tableResults, &datasync.SyncResult{
							Table: table,
							Error: err.Error(),
						})
					} else {
						log.Printf("[DataSync] Worker %d: Table %s preview: insert=%d, update=%d, delete=%d", workerID, table, result.Inserted, result.Updated, result.Deleted)
						tableResults = append(tableResults, result)
					}
				}

				targetDB.Close()

				var allSQL []string
				for _, r := range tableResults {
					if r.SQL != nil {
						allSQL = append(allSQL, r.SQL...)
					}
				}
				log.Printf("[DataSync] Worker %d: Generated %d SQL statements for target %s", workerID, len(allSQL), task.targetConfig.Name)

				resultsChan <- DataSyncResponse{
					TargetID:   task.targetID,
					TargetName: task.targetConfig.Name,
					Results:    tableResults,
				}
			}
			log.Printf("[DataSync] Worker %d finished", workerID)
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []DataSyncResponse
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

	log.Printf("[DataSync] Preview completed: %d targets processed", len(results))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(results)
}

func (h *DataSyncHandler) SyncDataStream(w http.ResponseWriter, r *http.Request) {
	var req DataSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}
	if err := validateDataSyncRequest(req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
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
		writeDataSyncSSE(w, flusher, event, data)
	}

	emitEvent("log", DataSyncLog{Level: dataSyncLogLevelInfo, Message: "已开始数据比对"})
	results, err := h.previewDataSync(req, func(progress DataSyncLog) {
		emitEvent("log", progress)
	})
	if err != nil {
		log.Printf("[DataSync] ERROR: %v", err)
		emitEvent("error", DataSyncLog{Level: dataSyncLogLevelError, Message: err.Error()})
		return
	}

	emitEvent("complete", map[string]interface{}{"completed": true, "results": results})
	return
}

// validateDataSyncRequest 确保每张待比对表都明确指定了非 id 的匹配字段。
func validateDataSyncRequest(req DataSyncRequest) error {
	if req.SourceID <= 0 || len(req.TargetIDs) == 0 || len(req.Tables) == 0 {
		return fmt.Errorf("source database, target database, and tables are required")
	}
	for _, table := range req.Tables {
		if req.KeyFields[table] == "" || strings.EqualFold(req.KeyFields[table], datasync.IgnoredIDColumn) {
			return fmt.Errorf("comparison field is required for table %s", table)
		}
	}
	return nil
}

func (h *DataSyncHandler) previewDataSync(req DataSyncRequest, progressFunc func(DataSyncLog)) ([]DataSyncResponse, error) {
	notifyDataSyncLog(progressFunc, DataSyncLog{Level: dataSyncLogLevelInfo, Message: "正在读取源数据库配置"})
	sourceConfig, err := audit.GetDBConfigByID(h.db, req.SourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source config: %w", err)
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not found")
	}

	notifyDataSyncLog(progressFunc, DataSyncLog{Level: dataSyncLogLevelInfo, Message: "正在连接源数据库"})
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

	targetConfigs := make([]*audit.DBConfig, 0, len(req.TargetIDs))
	for _, targetID := range req.TargetIDs {
		targetConfig, err := audit.GetDBConfigByID(h.db, targetID)
		if err != nil {
			log.Printf("[DataSync] ERROR: Failed to get target config for id=%d - %v", targetID, err)
			targetConfigs = append(targetConfigs, nil)
			continue
		}
		targetConfigs = append(targetConfigs, targetConfig)
	}

	type syncTask struct {
		targetConfig *audit.DBConfig
		targetID     int
	}

	concurrency := runtime.NumCPU() * dataSyncTargetWorkerMultiplier
	tasks := make(chan syncTask, len(targetConfigs))
	for i, targetConfig := range targetConfigs {
		tasks <- syncTask{targetConfig: targetConfig, targetID: req.TargetIDs[i]}
	}
	close(tasks)

	resultsChan := make(chan DataSyncResponse, len(targetConfigs))
	var waitGroup sync.WaitGroup
	for workerIndex := 0; workerIndex < concurrency; workerIndex++ {
		waitGroup.Add(1)
		go func(workerID int) {
			defer waitGroup.Done()
			for task := range tasks {
				if task.targetConfig == nil {
					resultsChan <- DataSyncResponse{TargetID: task.targetID, Error: "Target config not found"}
					continue
				}

				notifyDataSyncLog(progressFunc, DataSyncLog{
					Level:      dataSyncLogLevelInfo,
					TargetName: task.targetConfig.Name,
					Message:    "正在连接目标数据库",
				})
				targetDB := &dbcompare.DBConnection{
					DBType:   task.targetConfig.DBType,
					Host:     task.targetConfig.Host,
					Port:     task.targetConfig.Port,
					Database: task.targetConfig.Database,
					Username: task.targetConfig.Username,
					Password: task.targetConfig.Password,
				}
				if err := targetDB.Connect(); err != nil {
					resultsChan <- DataSyncResponse{
						TargetID:   task.targetID,
						TargetName: task.targetConfig.Name,
						Error:      "Failed to connect to target DB: " + err.Error(),
					}
					continue
				}

				var tableResults []*datasync.SyncResult
				for _, table := range req.Tables {
					notifyDataSyncLog(progressFunc, DataSyncLog{
						Level:      dataSyncLogLevelInfo,
						TargetName: task.targetConfig.Name,
						TableName:  table,
						Message:    "开始比对数据表",
					})
					result, err := datasync.PreviewTableDataWithProgress(sourceDB, targetDB, table, req.KeyFields[table], req.DeleteMissing, func(progress datasync.DataSyncProgress) {
						notifyDataSyncLog(progressFunc, DataSyncLog{
							Level:      dataSyncLogLevelInfo,
							TargetName: task.targetConfig.Name,
							TableName:  progress.TableName,
							Message:    progress.Message,
						})
					})
					if err != nil {
						notifyDataSyncLog(progressFunc, DataSyncLog{
							Level:      dataSyncLogLevelError,
							TargetName: task.targetConfig.Name,
							TableName:  table,
							Message:    err.Error(),
						})
						tableResults = append(tableResults, &datasync.SyncResult{Table: table, Error: err.Error()})
						continue
					}
					tableResults = append(tableResults, result)
				}

				targetDB.Close()
				resultsChan <- DataSyncResponse{
					TargetID:   task.targetID,
					TargetName: task.targetConfig.Name,
					Results:    tableResults,
				}
			}
		}(workerIndex)
	}

	go func() {
		waitGroup.Wait()
		close(resultsChan)
	}()

	var results []DataSyncResponse
	for result := range resultsChan {
		results = append(results, result)
	}
	sortDataSyncResultsByTargetID(results)
	return results, nil
}

func sortDataSyncResultsByTargetID(results []DataSyncResponse) {
	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].TargetID < results[i].TargetID {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func notifyDataSyncLog(progressFunc func(DataSyncLog), progress DataSyncLog) {
	if progressFunc != nil {
		progressFunc(progress)
	}
}

func writeDataSyncSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("[DataSync] ERROR: Failed to marshal SSE payload - %v", err)
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
	flusher.Flush()
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

// GetTableColumns 返回源表可用于数据比对的字段，id 不参与跨库数据匹配。
func (h *DataSyncHandler) GetTableColumns(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	configID, err := strconv.Atoi(vars["config_id"])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid config ID"})
		return
	}
	config, err := audit.GetDBConfigByID(h.db, configID)
	if err != nil || config == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Source config not found"})
		return
	}
	cacheKey := fmt.Sprintf("%d:%s", configID, vars["table_name"])
	if columns, ok := h.cachedColumns(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(columns)
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

	for _, table := range schema.Tables {
		if table.Name != vars["table_name"] {
			continue
		}
		columns := make([]string, 0, len(table.Columns))
		for _, column := range table.Columns {
			if !strings.EqualFold(column.Name, datasync.IgnoredIDColumn) {
				columns = append(columns, column.Name)
			}
		}
		h.cacheMu.Lock()
		h.columnsCache[cacheKey] = dataSyncColumnsCacheEntry{columns: append([]string(nil), columns...), expiresAt: time.Now().Add(dataSyncColumnsCacheTTL)}
		h.cacheMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(columns)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "Table not found"})
}

func (h *DataSyncHandler) cachedColumns(cacheKey string) ([]string, bool) {
	h.cacheMu.RLock()
	entry, ok := h.columnsCache[cacheKey]
	h.cacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return append([]string(nil), entry.columns...), true
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

// TestConnection 测试已保存或新填写的数据库连接参数是否可用。
func (h *DataSyncHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var request testConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	config := request.DBConfig
	if request.ConfigID > 0 {
		storedConfig, err := audit.GetDBConfigByID(h.db, request.ConfigID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Failed to get database config"})
			return
		}
		if storedConfig == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Database config not found"})
			return
		}
		config = *storedConfig
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
