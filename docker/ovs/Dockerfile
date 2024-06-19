FROM debian:bookworm-slim


# Install dependencies
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -y && \
    apt-get install -y python3 python3-cmd2 less vim tcpdump \
	    iputils-ping iputils-arping iputils-tracepath net-tools \
	    traceroute htop procps openvswitch-switch \
	    ethtool && rm -rf /var/lib/apt/lists/*

# Create database and pid file directory
RUN /usr/bin/ovsdb-tool create /etc/openvswitch/conf.db
RUN mkdir -pv /var/run/openvswitch/

# Add scripts to manage ovs
ADD ovs-console.py /usr/bin/ovs-console.py
RUN chmod +x /usr/bin/ovs-console.py
ADD ovs-config.py /usr/bin/ovs-config.py
RUN chmod +x /usr/bin/ovs-config.py

# Add script to launch ovs
ADD run-ovs.sh /bin/run-ovs.sh
RUN chmod +x /bin/run-ovs.sh

CMD ["/bin/run-ovs.sh"]
