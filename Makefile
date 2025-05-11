migration_up:
	docker exec -it teobot-db-1 /db/migrate -path /db/migrations -database "mysql://teobot:teo@tcp(127.0.0.1)/teobot" up

migration_down:
	docker exec -it teobot-db-1 /db/migrate -path /db/migrations -database "mysql://teobot:teo@tcp(127.0.0.1)/teobot" down

dump_db_schema:
	./dump_schema.sh