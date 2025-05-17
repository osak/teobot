migration_up:
	migrate -path db/migrations -database "postgres://teobot:teo@127.0.0.1/teobot?sslmode=disable&x-multi-statement=true" up

migration_down:
	migrate -path db/migrations -database "postgres://teobot:teo@127.0.0.1/teobot?sslmode=disable&x-multi-statement=true" down

dump_db_schema:
	./dump_schema.sh