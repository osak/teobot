#!/bin/bash

docker build --build-arg BUILD_TIMESTAMP=$(date +%s) -t teobot:latest .
