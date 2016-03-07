default:
	go build .

test:
	go test ./...

imports:
	goimports -w ./..

lint:
	golint ./...
