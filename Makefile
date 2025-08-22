.PHONY: all build_chat_history migration_up migration_down bin/teobot

all: bin/teobot

bin/teobot:
	go build -o bin/teobot cmd/teobot/main.go

build_chat_history:
	go build -o bin/build_chat_history cmd/build_chat_history/main.go

migration_up:
	migrate -path db/migrations -database "postgres://teobot:teo@127.0.0.1/teobot?sslmode=disable" up

migration_down:
	migrate -path db/migrations -database "postgres://teobot:teo@127.0.0.1/teobot?sslmode=disable" down 1
