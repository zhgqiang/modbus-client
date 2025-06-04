#!/bin/bash

# 设置编译目标为Linux amd64
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

go build -tags netgo -v -o modbusrtu cmd/rtu/main.go
