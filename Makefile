
VERSION := $(shell cat VERSION)
INSTALLDIR := /usr/local/bin
CONFDIR := /etc/gonetem

proto: internal/proto/netem.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/netem.proto

build-console:
	go build -o ./bin/gonetem-console-linux-amd64 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-console/main.go
	# GOOS=windows GOARCH=amd64 go build \
	# 	-o ./bin/gonetem-console-windows-amd64.exe \
	# 	-ldflags "-X main.Version=$(VERSION)" \
	# 	cmd/gonetem-console/main.go
	# GOOS=darwin GOARCH=amd64 go build \
	# 	-o ./bin/gonetem-console-darwin-amd64.exe \
	#	-ldflags "-X main.Version=$(VERSION)" \
	#	cmd/gonetem-console/main.go

build-server:
	go build -o ./bin/gonetem-server \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-server/main.go

build: build-console build-server

build-console-pi:
	# build for raspberry pi 4
	env GOOS=linux GOARCH=arm GOARM=7 go build -o ./bin/gonetem-console_armv7 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-console/main.go

build-server-pi:
	# build for raspberry pi 4
	env GOOS=linux GOARCH=arm GOARM=7 go build -o ./bin/gonetem-server_armv7 \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/gonetem-server/main.go

build-pi: build-console-pi build-server-pi

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

install: clean build
	@echo "installing gonetem-console/gonetem-server in '${INSTALLDIR}' directory"
	cp bin/gonetem-server ${INSTALLDIR}
	cp bin/gonetem-console ${INSTALLDIR}
	mkdir -p ${CONFDIR}
	cp conf/config.yaml ${CONFDIR}

uninstall:
	@echo "delete gonetem-console/gonetem-server in '${INSTALLDIR}' directory"
	rm ${INSTALLDIR}/gonetem-console
	rm ${INSTALLDIR}/gonetem-server
	rm ${CONFDIR}/config.yaml
	rmdir ${CONFDIR}


.PHONY: clean proto build build-deb install
