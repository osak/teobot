migration_up:
	migrate -path db/migrations -database "postgres://teobot:teo@127.0.0.1/teobot?sslmode=disable" up

migration_down:
	migrate -path db/migrations -database "postgres://teobot:teo@127.0.0.1/teobot?sslmode=disable" down 1