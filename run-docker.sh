#!/bin/bash

docker run -d -v "$(pwd)/.env.docker:/opt/teobot/.env" -v "$(pwd)/data:/opt/teobot/data" --name teobot teobot:latest
