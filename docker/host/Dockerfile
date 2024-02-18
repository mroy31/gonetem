FROM golang:1.21

# download quic-go
WORKDIR /go/src
RUN git clone https://github.com/mroy31/quic-go-example

# build quic client
WORKDIR /go/src/quic-go-example
RUN go get -d ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o /quic-client ./cmd/client/main.go

FROM debian:bookworm-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get install -y python3 python3-pyroute2 less nano vim tcpdump \
	    iputils-ping iputils-arping iputils-tracepath net-tools file \
	    telnet man traceroute hping3 iptables links htop ntp git wget at \
	    snmp iperf3 build-essential curl tftp-hpa isc-dhcp-client ethtool \
		bind9-dnsutils && rm -rf /var/lib/apt/lists/*

## install quic-go client
COPY --from=0 /quic-client /usr/local/bin/quic-client

## install multicast programs
COPY ./programs/mult-receive.py /usr/bin/mult_receive
RUN chmod +x /usr/bin/mult_receive
COPY ./programs/mult-send.py /usr/bin/mult_send
RUN chmod +x /usr/bin/mult_send

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py

COPY ./scripts/network-config.py /usr/bin/
RUN chmod +x /usr/bin/network-config.py

COPY ./conf/dhclient-dnsupdate /etc/dhcp/dhclient-enter-hooks.d/dnsupdate
RUN chmod +x /etc/dhcp/dhclient-enter-hooks.d/dnsupdate

WORKDIR /root
CMD ["/usr/bin/iinit.py"]
