
VERSION := $(shell cat VERSION)
INSTALLDIR := /usr/local/bin
CONFDIR := /etc/gonetem
DEB_OPTIONS := --input-type dir --name gonetem --version $(VERSION) \
		--description "gonetem is a network emulator written in go" \
		--maintainer "<mickael.royer@enac.fr>" \
		--config-files /etc/gonetem/config.yaml \
		--deb-systemd conf/gonetem.service\
		--deb-systemd-auto-start --deb-systemd-enable \
		--deb-recommends xterm --deb-recommends wireshark \
		--deb-recommends "docker-ce | docker.io"

proto: internal/proto/netem.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/netem.proto

build-console-amd64:
	env GOOS=linux GOARCH=amd64 go build -o ./bin/gonetem-console_amd64 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-console/main.go

build-server-amd64:
	env GOOS=linux GOARCH=amd64 go build -o ./bin/gonetem-server_amd64 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-server/main.go

build-amd64: build-console-amd64 build-server-amd64

build-console-armv7:
	env GOOS=linux GOARCH=arm GOARM=7 go build -o ./bin/gonetem-console_armv7 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-console/main.go

build-server-armv7:
	env GOOS=linux GOARCH=arm GOARM=7 go build -o ./bin/gonetem-server_armv7 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-server/main.go

build-armv7: build-console-armv7 build-server-armv7

build-console-arm64:
	env GOOS=linux GOARCH=arm64 go build -o ./bin/gonetem-console_arm64 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-console/main.go

build-server-arm64:
	env GOOS=linux GOARCH=arm64 go build -o ./bin/gonetem-server_arm64 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-server/main.go

build-arm64: build-console-arm64 build-server-arm64

build-console-darwin-arm64:
	env GOOS=darwin GOARCH=arm64 go build -o ./bin/gonetem-console_darwin_arm64 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-console/main.go

build-deb-amd64: clean build-amd64
	docker run -v $(shell pwd):/src --rm gonetem-build fpm --output-type deb \
		--architecture=amd64 \
		${DEB_OPTIONS} \
		bin/gonetem-server_amd64=/usr/sbin/gonetem-server \
		bin/gonetem-console_amd64=/usr/bin/gonetem-console \
		conf/config.yaml=/etc/gonetem/ \

build-deb-armv7: clean build-armv7
	docker run -v $(shell pwd):/src --rm gonetem-build fpm --output-type deb \
		--architecture=armhf \
		${DEB_OPTIONS} \
		bin/gonetem-server_armv7=/usr/sbin/gonetem-server \
		bin/gonetem-console_armv7=/usr/bin/gonetem-console \
		conf/config.yaml=/etc/gonetem/ \

build-deb-arm64: clean build-arm64
	docker run -v $(shell pwd):/src --rm gonetem-build fpm --output-type deb \
		--architecture=arm64 \
		${DEB_OPTIONS} \
		bin/gonetem-server_arm64=/usr/sbin/gonetem-server \
		bin/gonetem-console_arm64=/usr/bin/gonetem-console \
		conf/config.yaml=/etc/gonetem/ \

clean:
	rm -rf bin

install-amd64: clean build-amd64
	@echo "installing gonetem-console/gonetem-server in '${INSTALLDIR}' directory"
	cp bin/gonetem-server_amd64 ${INSTALLDIR}/gonetem-server
	cp bin/gonetem-console_amd64 ${INSTALLDIR}/gonetem-console
	mkdir -p ${CONFDIR}
	cp conf/config.yaml ${CONFDIR}

install-arm64: clean build-arm64
	@echo "installing gonetem-console/gonetem-server in '${INSTALLDIR}' directory"
	cp bin/gonetem-server_arm64 ${INSTALLDIR}/gonetem-server
	cp bin/gonetem-console_arm64 ${INSTALLDIR}/gonetem-console
	mkdir -p ${CONFDIR}
	cp conf/config.yaml ${CONFDIR}

uninstall:
	@echo "delete gonetem-console/gonetem-server in '${INSTALLDIR}' directory"
	rm ${INSTALLDIR}/gonetem-console
	rm ${INSTALLDIR}/gonetem-server
	rm ${CONFDIR}/config.yaml
	rmdir ${CONFDIR}

test:
	sudo go test ./...

.PHONY: clean proto build-amd64 build-amrv7 build-arm64 build-deb-amd64 build-deb-arm64 install-amd64 install-arm64
