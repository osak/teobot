#!/bin/bash

# Script to dump all table schemas from the MySQL database
echo "Dumping database schema to go/gen/schema.sql..."

# Run mysqldump with no-data option to get only CREATE TABLE statements
docker exec teobot-db-1 mariadb-dump --no-data --compact --skip-triggers --user=teobot --password=teo --host=127.0.0.1 teobot \
    | sed -e 's/ COLLATE=\(.*\);/;/'> go/gen/schema.sql

# Check if the dump was successful
if [ $? -eq 0 ]; then
  echo "Database schema successfully dumped to go/gen/schema.sql"
else
  echo "Error: Failed to dump database schema"
  exit 1
fi