all: zdns

zdns:
	CGO_ENABLED="0" go build -trimpath

clean:
	rm -f zdns

install: zdns
	go install

.PHONY: zdns clean

fmt:
	goimports -l -w .
	gofmt -s -w .

lint: fmt
	go vet ./...