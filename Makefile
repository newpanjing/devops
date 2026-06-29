.PHONY: build run dev clean

build:
	mkdir -p cmd/server/static
	cp frontend/src/index.html cmd/server/static/
	cp frontend/src/app.js cmd/server/static/
	go build -o dbsync cmd/server/main.go

run: build
	./dbsync

dev:
	mkdir -p cmd/server/static
	cp frontend/src/index.html cmd/server/static/
	cp frontend/src/app.js cmd/server/static/
	go run cmd/server/main.go

clean:
	rm -rf cmd/server/static
	rm -f dbsync
	rm -rf data
