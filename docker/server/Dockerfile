FROM debian:buster-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get install -y python3 python3-pyroute2 less nano vim tcpdump \
	    iputils-ping iputils-arping iputils-tracepath net-tools file \
		telnet man traceroute hping3 iptables links lighttpd htop \
		ntp snmp isc-dhcp-server tftpd-hpa iproute2 iperf3 ethtool \
        && rm -rf /var/lib/apt/lists/*

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py

COPY ./scripts/network-config.py /usr/bin/
RUN chmod +x /usr/bin/network-config.py

RUN touch /var/lib/dhcp/dhcpd.leases

CMD ["/usr/bin/iinit.py"]
