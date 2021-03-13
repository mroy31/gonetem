FROM alpine:latest


# Install dependencies
RUN apk --update add \
    bash dumb-init openvswitch \
    htop net-tools tcpdump less vim \
    iputils procps python3 shadow ethtool \
    py3-pip

# Install python dependencies
RUN pip3 install cmd2

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


ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/bin/run-ovs.sh"]
