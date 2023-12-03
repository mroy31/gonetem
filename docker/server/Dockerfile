FROM golang:1.20

# download quic-go
WORKDIR /go/src
RUN git clone --depth 1 --branch v0.40.0 https://github.com/quic-go/quic-go.git

# build quic client / server
WORKDIR /go/src/quic-go
RUN CGO_ENABLED=0 GOOS=linux go build -o /quic-server ./example/main.go


FROM debian:bookworm-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get install -y python3 python3-pyroute2 less nano vim tcpdump \
	iputils-ping iputils-arping iputils-tracepath net-tools file \
	telnet man traceroute hping3 iptables links lighttpd htop \
	ntp snmp isc-dhcp-server tftpd-hpa iproute2 iperf3 ethtool git \
	isc-dhcp-relay bind9 bind9-dnsutils && rm -rf /var/lib/apt/lists/*

## install quic-go server
# install certs
COPY ./certs/* /certs/
RUN ln -sf /certs/ca.pem /etc/ssl/certs/Quic-Go.pem
# copy quic binaries
COPY --from=0 /quic-server /usr/local/bin/quic-server

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py

COPY ./scripts/network-config.py /usr/bin/
RUN chmod +x /usr/bin/network-config.py

RUN touch /var/lib/dhcp/dhcpd.leases

CMD ["/usr/bin/iinit.py"]
