FROM debian:bookworm-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND noninteractive
ENV APT_KEY_DONT_WARN_ON_DANGEROUS_USAGE=DontWarn

ARG REPO_KEY=frr.repo.gpg
ARG SOURCE_LIST=frr.source.list

## add FRR repository
RUN apt-get update -y && apt-get install -y gnupg ca-certificates curl
RUN curl -s https://deb.frrouting.org/frr/keys.gpg \
	| tee /usr/share/keyrings/frrouting.gpg > /dev/null
RUN echo deb '[signed-by=/usr/share/keyrings/frrouting.gpg]' https://deb.frrouting.org/frr \
     bookworm frr-stable | tee -a /etc/apt/sources.list.d/frr.list


## install FRR and tools
RUN apt-get update -y && \
    apt-get install -y frr frr-pythontools frr-snmp ethtool \
		curl less nano vim psmisc tcpdump iputils-ping \
		iputils-arping iputils-tracepath net-tools file htop procps \
        && rm -rf /var/lib/apt/lists/*

# configure FRR
COPY conf/frr.conf /etc/frr
COPY conf/daemons /etc/frr
RUN chown -R frr:frr /etc/frr
RUN mkdir /var/run/frr && chown frr:frr /var/run/frr
RUN gpasswd -a root frr

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py
CMD ["/usr/bin/iinit.py"]
