run:
	go run cmd/web/*.go
build: //make sure app.inProduction is set to true
	go build cmd/web/*.go