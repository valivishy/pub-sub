.PHONY: lint run start

lint:
	golangci-lint run ./...

run:
	osascript -e 'tell application "Terminal" to do script "cd \"$(PWD)\" && go run ./cmd/server"'

	osascript -e 'tell application "Terminal" to do script "cd \"$(PWD)\" && go run ./cmd/client"'

	osascript -e 'tell application "Terminal" to do script "cd \"$(PWD)\" && go run ./cmd/client"'

	@echo "Server and clients started in separate terminal tabs"

start:
	tmux new-session -d -s pubsub_server 'go run ./cmd/server; read'
	tmux split-window -h -t pubsub_server 'printf "washington\n" && go run ./cmd/client'
	tmux split-window -v -t pubsub_server 'printf "napoleon\n" && go run ./cmd/client'
	tmux select-layout -t pubsub_server tiled
	tmux attach-session -t pubsub_server
