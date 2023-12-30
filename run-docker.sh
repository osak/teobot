#!/bin/bash

docker run -d -v "$(pwd)/data:/opt/teobot/data" --name teobot teobot:latest
