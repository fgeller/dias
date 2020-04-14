export SHELL:=/usr/bin/env bash -O extglob -c
export GO111MODULE:=on
export OS=$(shell uname | tr '[:upper:]' '[:lower:]')
export ARTIFACT=dias

build: GOOS ?= ${OS}
build: GOARCH ?= amd64
build: clean
	GOOS=${GOOS} GOARCH=${GOARCH} go build -v -o ${ARTIFACT} .

test: clean
	go test -v -vet=all -failfast

clean:
	rm -f ${ARTIFACT}
	rm -f ${ARTIFACT}-*.txz

run: build
	./${ARTIFACT}
