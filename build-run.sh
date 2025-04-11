#!/usr/bin/env bash

go build -o ./build/default
chmod +x ./build/default
./build/default "$@"