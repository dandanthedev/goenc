#!/bin/bash

cd ui
bun run build
rm -rf ../dist
mv dist ../
cd ..

#build server to ./bin
go build -o ./bin/goenc ./main.go