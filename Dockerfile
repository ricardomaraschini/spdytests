FROM	golang:1.13 AS builder
RUN	go get -u github.com/shuLhan/go-bindata/...
WORKDIR	/spdytests
COPY	. .
RUN	go-bindata -pkg bindata -o bindata/bindata.go assets/*
RUN	go build -mod vendor -o /usr/local/bin/spdytests ./cmd/

FROM	centos:latest  
RUN 	yum install socat -y
COPY	--from=builder /usr/local/bin/spdytests /usr/local/bin/
EXPOSE	8080
