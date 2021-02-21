
proto: internal/proto/netem.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/netem.proto

build-console:
	go build -o ./bin/gonetem-console cmd/gonetem-console/main.go

build-server:
	go build -o ./bin/gonetem-server cmd/gonetem-server/main.go

build: build-console build-server

clean:
	rm -r bin
