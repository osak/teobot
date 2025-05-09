migration_up:
	docker exec teobot-db-1 /db/migrate -path /db/migrations -database "mysql://teobot:teo@tcp(127.0.0.1)/teobot" up