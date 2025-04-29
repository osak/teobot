#!/bin/bash

docker run --restart always -d -v "$(pwd)/data:/opt/teobot/data" -v "$(pwd)/tmp:/opt/teobot/tmp" --name teobot teobot:latest
