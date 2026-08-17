package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"dbsync/internal/audit"
	"dbsync/internal/auth"
	"dbsync/internal/handlers"
)

//go:embed static/*
var staticFiles embed.FS

var (
	dbPath = flag.String("db", "./data/dbsync.db", "Path to SQLite database")
	port   = flag.String("port", "8080", "Server port")
)

func main() {
	flag.Parse()

	db, err := audit.InitDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := auth.InitializeAdminUser(db); err != nil {
		log.Printf("Failed to initialize admin user: %v", err)
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static filesystem: %v", err)
	}

	muxRouter := mux.NewRouter()

	api := muxRouter.PathPrefix("/api").Subrouter()

	authHandler := handlers.NewAuthHandler(db)
	api.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	dataSyncHandler := handlers.NewDataSyncHandler(db)
	api.HandleFunc("/datasync/test", dataSyncHandler.TestConnection).Methods("POST")

	apiWithAuth := api.PathPrefix("").Subrouter()
	apiWithAuth.Use(auth.AuthMiddleware)

	apiWithAuth.HandleFunc("/auth/validate", authHandler.ValidateToken).Methods("GET")
	apiWithAuth.HandleFunc("/auth/user", authHandler.GetCurrentUser).Methods("GET")

	dbConfigHandler := handlers.NewDBConfigHandler(db)
	apiWithAuth.HandleFunc("/dbconfig", dbConfigHandler.ListConfigs).Methods("GET")
	apiWithAuth.HandleFunc("/dbconfig/{id}", dbConfigHandler.GetConfig).Methods("GET")
	apiWithAuth.HandleFunc("/dbconfig", dbConfigHandler.CreateConfig).Methods("POST")
	apiWithAuth.HandleFunc("/dbconfig/{id}", dbConfigHandler.DeleteConfig).Methods("DELETE")

	schemaHandler := handlers.NewSchemaHandler(db)
	apiWithAuth.HandleFunc("/schema/tables/{config_id}", schemaHandler.GetTables).Methods("GET")

	apiWithAuth.HandleFunc("/datasync/sync/stream", dataSyncHandler.SyncDataStream).Methods("POST")
	apiWithAuth.HandleFunc("/datasync/preview/{config_id}/{table_name}", dataSyncHandler.GetTablePreview).Methods("GET")
	apiWithAuth.HandleFunc("/datasync/count/{config_id}/{table_name}", dataSyncHandler.GetTableCount).Methods("GET")

	fileServer := http.FileServer(http.FS(staticFS))

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			muxRouter.ServeHTTP(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if strings.Contains(path, ".") {
			fileServer.ServeHTTP(w, r)
			return
		}

		handlers.HandleIndex(http.FS(staticFS))(w, r)
	}))

	fmt.Printf("Server starting on http://localhost:%s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
