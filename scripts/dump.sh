#!/bin/bash

set -eu

mkdir -p data
pg_dump -h 127.0.0.1 -U teobot -d teobot --no-owner > data/dump.sql