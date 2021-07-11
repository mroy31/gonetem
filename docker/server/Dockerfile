FROM golang:1.15

# download quic-go
WORKDIR /go/src
RUN git clone https://github.com/mroy31/quic-go.git

# build quic client / server
WORKDIR /go/src/quic-go
RUN CGO_ENABLED=0 GOOS=linux go build -o /quic-server ./enac/server/server.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /quic-client ./enac/client/client.go


FROM debian:buster-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get install -y python3 python3-pyroute2 less nano vim tcpdump \
	iputils-ping iputils-arping iputils-tracepath net-tools file \
	telnet man traceroute hping3 iptables links lighttpd htop \
	ntp snmp isc-dhcp-server tftpd-hpa iproute2 iperf3 ethtool git \
        && rm -rf /var/lib/apt/lists/*

## install quic-go
# install certs
COPY ./certs/* /certs/
RUN ln -sf /certs/ca.pem /etc/ssl/certs/Quic-Go.pem
# copy quic binaries
COPY --from=0 /quic-server /usr/local/bin/quic-server
COPY --from=0 /quic-client /usr/local/bin/quic-client

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py

COPY ./scripts/network-config.py /usr/bin/
RUN chmod +x /usr/bin/network-config.py

RUN touch /var/lib/dhcp/dhcpd.leases

CMD ["/usr/bin/iinit.py"]
