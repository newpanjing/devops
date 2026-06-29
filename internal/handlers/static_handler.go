package handlers

import (
	"net/http"
	"path"
)

func HandleIndex(staticFS http.FileSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("index.html")
		if err != nil {
			http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "text/html")
		stat, _ := file.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), file)
	}
}

func getContentType(filePath string) string {
	switch path.Ext(filePath) {
	case ".html":
		return "text/html"
	case ".js":
		return "application/javascript"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
