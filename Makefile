.PHONY: all build_chat_history migration_up migration_down bin/teobot

all: bin/teobot

bin/teobot:
	GOEXPERIMENT=jsonv2 go build -o bin/teobot cmd/teobot_new/main.go

build_chat_history:
	go build -o bin/build_chat_history cmd/build_chat_history/main.go

migration_up:
	docker run -v "$(shell pwd)/db/migrations:/migrations" --network teobot_default migrate/migrate -path /migrations -database "postgres://teobot:teo@db/teobot?sslmode=disable" up
	docker run -v "$(shell pwd)/db/migrations:/migrations" --network teobot_default migrate/migrate -path /migrations -database "postgres://teobot_testing:teo@db/teobot_testing?sslmode=disable" up

migration_down:
	docker run -v "$(shell pwd)/db/migrations:/migrations" --network teobot_default migrate/migrate -path /migrations -database "postgres://teobot:teo@db/teobot?sslmode=disable" down 1
	docker run -v "$(shell pwd)/db/migrations:/migrations" --network teobot_default migrate/migrate -path /migrations -database "postgres://teobot_testing:teo@db/teobot_testing?sslmode=disable" down 1

sqlc:
	docker run --rm -v $(shell pwd):/src -w /src sqlc/sqlc generate
