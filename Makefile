.PHONY: lint run

lint:
	golangci-lint run ./...

run:
	osascript -e 'tell application "Terminal" to do script "cd \"$(PWD)\" && go run ./cmd/server"'

	osascript -e 'tell application "Terminal" to do script "cd \"$(PWD)\" && go run ./cmd/client"'

	osascript -e 'tell application "Terminal" to do script "cd \"$(PWD)\" && go run ./cmd/client"'

	@echo "Server and clients started in separate terminal tabs"
