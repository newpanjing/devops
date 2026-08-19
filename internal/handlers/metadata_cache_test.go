package handlers

import (
	"sync"
	"testing"
	"time"

	"dbsync/internal/dbcompare"
)

// TestMetadataCache verifies metadata is copied on cache hits and expires at its deadline.
func TestMetadataCache(t *testing.T) {
	now := time.Now()
	schemaHandler := &SchemaHandler{tablesCache: map[int]schemaTablesCacheEntry{1: {tables: []string{"users"}, expiresAt: now.Add(time.Minute)}}}
	tables, ok := schemaHandler.cachedTables(1)
	if !ok || tables[0] != "users" {
		t.Fatal("expected table cache hit")
	}
	tables[0] = "changed"
	if cached, _ := schemaHandler.cachedTables(1); cached[0] != "users" {
		t.Fatal("cache hit must return a copy")
	}

	dataHandler := &DataSyncHandler{columnsCache: map[string]dataSyncColumnsCacheEntry{"1:users": {columns: []string{"code"}, expiresAt: now.Add(-time.Second)}}, cacheMu: sync.RWMutex{}}
	if _, ok := dataHandler.cachedColumns("1:users"); ok {
		t.Fatal("expired column cache must miss")
	}
}

func TestFilterSchemaTables(t *testing.T) {
	schema := &dbcompare.Schema{Tables: []dbcompare.Table{{Name: "users"}, {Name: "orders"}}}
	filtered, found := filterSchemaTables(schema, []string{"orders"})
	if found != 1 || len(filtered.Tables) != 1 || filtered.Tables[0].Name != "orders" {
		t.Fatalf("unexpected filtered schema: found=%d tables=%v", found, filtered.Tables)
	}
}
