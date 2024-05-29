#!/bin/bash

docker run --restart always -d -v "$(pwd)/data:/opt/teobot/data" --name teobot teobot:latest
