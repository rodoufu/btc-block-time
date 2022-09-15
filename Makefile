build:
	cd cmd/blocktime && go build

clean:
	rm cmd/blocktime/blocktime

test:
	go test -v ./...
