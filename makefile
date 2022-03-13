BINARY_NAME=p2pchord

HOME:=$(shell pwd)
OS_NAME := $(shell uname -s | tr A-Z a-z)

os:
	@echo $(OS_NAME)

install:
	brew install golang

fmt: go fmt

build:
	GOARCH=amd64 GOOS=darwin go build -o ${BINARY_NAME} main.go
    GOARCH=amd64 GOOS=linux go build -o ${BINARY_NAME} main.go
    GOARCH=amd64 GOOS=window go build -o ${BINARY_NAME} main.go

run:
	./${BINARY_NAME}

build_and_run: build run

clean:
	go clean
	rm ${BINARY_NAME}

.PHONY: all p2pchord