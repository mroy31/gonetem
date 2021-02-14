
proto: internal/proto/netem.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/netem.proto

build-emulator:
	go build -o ./gonetem-emulator cmd/gonetem-emulator/main.go
