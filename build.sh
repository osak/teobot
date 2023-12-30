#!/bin/bash

mkdir -p build
npm run build-env-file env.json build/env.json
docker build -t teobot:latest .
