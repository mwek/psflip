psflip: Makefile $(wildcard ./cmd/*) $(wildcard ./pkg/**/*)
	go build -o psflip -ldflags="-s -w" ./cmd
