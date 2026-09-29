.PHONY: build run

build:
	go build -o tiered-io .

run: build
	./tiered-io
