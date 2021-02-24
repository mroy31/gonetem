FROM alpine:latest


# Install dependencies
RUN apk --update add \
    bash dumb-init openvswitch \
    htop net-tools tcpdump less vim \
    iputils procps python3 shadow ethtool


# Create database and pid file directory
RUN /usr/bin/ovsdb-tool create /etc/openvswitch/conf.db
RUN mkdir -pv /var/run/openvswitch/


# Add script to launch ovs
ADD run-ovs.sh /bin/run-ovs.sh
RUN chmod +x /bin/run-ovs.sh


ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/bin/run-ovs.sh"]
