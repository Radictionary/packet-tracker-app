run:
	go run cmd/web/*.go
build: #make sure app.inProduction is set to true in the main.go file
	go build cmd/web/*.go