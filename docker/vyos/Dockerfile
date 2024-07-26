from vyos:1.4

COPY config/config.boot /opt/vyatta/etc/config/config.boot

COPY scripts/save-vyos-config.sh /usr/bin
COPY scripts/start-vyos.sh /usr/bin
RUN chmod +x /usr/bin/start-vyos.sh
RUN chmod +x /usr/bin/save-vyos-config.sh

RUN rm /etc/systemd/system/vyos.target.wants/*

ENTRYPOINT /sbin/init
