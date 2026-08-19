package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"dbsync/internal/audit"
)

// TestConnectionUsesSavedConfig 验证配置列表的 config_id 请求会读取已保存的连接参数。
func TestConnectionUsesSavedConfig(t *testing.T) {
	db, err := audit.InitDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := audit.CreateDBConfig(db, &audit.DBConfig{Name: "test", DBType: "sqlite", Database: ":memory:"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/datasync/test", bytes.NewBufferString(`{"config_id":1}`))
	response := httptest.NewRecorder()
	NewDataSyncHandler(db).TestConnection(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}
