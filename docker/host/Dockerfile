FROM golang:1.17

# download quic-go
WORKDIR /go/src
RUN git clone https://github.com/mroy31/quic-go.git

# build quic client / server
WORKDIR /go/src/quic-go
RUN CGO_ENABLED=0 GOOS=linux go build -o /quic-client ./enac/client/client.go

FROM debian:bullseye-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get install -y python3 python3-pip less nano vim tcpdump \
	    iputils-ping iputils-arping iputils-tracepath net-tools file \
	    telnet man traceroute hping3 iptables links htop ntp git wget at \
	    snmp iperf3 build-essential curl tftp-hpa isc-dhcp-client ethtool \
        && rm -rf /var/lib/apt/lists/*

## install recent version of pyroute2
RUN pip3 install pyroute2==0.7.2

## build and install multicast programs
COPY ./programs/mult_receive.c /tmp/
COPY ./programs/mult_send.c /tmp/
RUN gcc -o /usr/bin/mult_receive /tmp/mult_receive.c
RUN gcc -o /usr/bin/mult_send /tmp/mult_send.c
RUN rm /tmp/mult_receive.c /tmp/mult_send.c

## cleanup build env
RUN apt-get remove -y build-essential
RUN apt autoremove -y build-essential

## install quic-go
# install certs
COPY ./certs/* /certs/
RUN ln -sf /certs/ca.pem /etc/ssl/certs/Quic-Go.pem
# copy quic binaries
COPY --from=0 /quic-client /usr/local/bin/quic-client

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py

COPY ./scripts/network-config.py /usr/bin/
RUN chmod +x /usr/bin/network-config.py

COPY ./conf/dhclient-dnsupdate /etc/dhcp/dhclient-enter-hooks.d/dnsupdate
RUN chmod +x /etc/dhcp/dhclient-enter-hooks.d/dnsupdate

WORKDIR /root
CMD ["/usr/bin/iinit.py"]
