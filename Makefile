.PHONY: build install clean

build:
	go build -o ecsx ./cmd/ecsx

install:
	go install ./cmd/ecsx

clean:
	rm -f ecsx
