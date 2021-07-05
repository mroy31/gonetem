FROM debian:buster-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get install -y python3 python3-pyroute2 less nano vim tcpdump \
	iputils-ping iputils-arping iputils-tracepath net-tools file \
	telnet man traceroute hping3 iptables links lighttpd htop \
	ntp snmp isc-dhcp-server tftpd-hpa iproute2 iperf3 ethtool git \
        && rm -rf /var/lib/apt/lists/*

## build and install quic-go
## first install buster backports repo for golang 1.15
RUN echo "deb http://deb.debian.org/debian buster-backports main" >> /etc/apt/sources.list \
	&& apt-get update -y \
	&& apt-get install -t buster-backports -y golang \
        && rm -rf /var/lib/apt/lists/*
# install certs
COPY ./certs/* /certs/
RUN ln -sf /certs/ca.pem /etc/ssl/certs/Quic-Go.pem
# compile quic-go client/server
WORKDIR /tmp
RUN git clone https://github.com/mroy31/quic-go.git \
	&& cd quic-go \
	&& go build -o /usr/local/bin/quic-server ./enac/server/server.go \
	&& go build -o /usr/local/bin/quic-client ./enac/client/client.go \
	&& rm -r /tmp/quic-go && rm -r /root/go && rm -r /root/.cache

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py

COPY ./scripts/network-config.py /usr/bin/
RUN chmod +x /usr/bin/network-config.py

RUN touch /var/lib/dhcp/dhcpd.leases

CMD ["/usr/bin/iinit.py"]
