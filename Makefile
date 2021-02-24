
VERSION := $(shell cat VERSION)

proto: internal/proto/netem.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/netem.proto

build-console:
	go build -o ./bin/gonetem-console \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-console/main.go

build-server:
	go build -o ./bin/gonetem-server \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-server/main.go

build: build-console build-server

build-deb: clean build
	docker run -v $(shell pwd):/src --rm gonetem-build fpm --output-type deb \
		--input-type dir --name gonetem --version $(VERSION) \
		--maintainer "<mickael.royer@enac.fr>" \
		--config-files /etc/gonetem/config.yaml \
		--deb-systemd conf/gonetem.service\
		--deb-recommends xterm --deb-recommends wireshark \
		--deb-recommends docker-ce \
		bin/gonetem-server=/usr/sbin/ \
		bin/gonetem-console=/usr/bin/ \
		conf/config.yaml=/etc/gonetem/ \

clean:
	rm -rf bin
