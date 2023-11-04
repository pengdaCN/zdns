all: zdns

zdns:
	CGO_ENABLED="0" go build -trimpath

clean:
	rm -f zdns

install: zdns
	go install

.PHONY: zdns clean

