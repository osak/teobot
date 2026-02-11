#!/bin/bash

set -eu

echo "
DROP DATABASE teobot_testing;
CREATE DATABASE teobot_testing;
CREATE USER teobot_testing PASSWORD 'teo';

\c teobot_testing
GRANT ALL ON DATABASE teobot_testing TO teobot_testing;
GRANT ALL ON SCHEMA public TO teobot_testing;
" | psql -h 127.0.0.1 -U teobot

PGPASSWORD=teo psql -v ON_ERROR_STOP=1 -h 127.0.0.1 -U teobot_testing -d teobot_testing -1 < data/dump.sql