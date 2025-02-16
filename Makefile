all: psflip pidwatch

psflip: Makefile $(wildcard ./cmd/psflip/*) $(wildcard ./pkg/**/*)
	go build -o psflip -ldflags="-s -w" ./cmd/psflip

pidwatch: Makefile $(wildcard ./cmd/pidwatch/*) $(wildcard ./pkg/**/*)
	go build -o pidwatch -ldflags="-s -w" ./cmd/pidwatch
