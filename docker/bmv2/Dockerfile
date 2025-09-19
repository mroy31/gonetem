FROM debian:bookworm AS base-builder

ARG MAKEFLAGS=-j4
ENV DEBIAN_FRONTEND=noninteractive
ENV BUILDER_DEPS automake \
            gnupg \
            ca-certificates \
            curl \
            build-essential \
            g++ \
            clang-13 \
            libboost-dev \
            bison \
            flex \
            git \
            cmake \
            lcov \
            libgmp-dev \
            libpcap-dev \
            libnanomsg-dev \
            libthrift-dev \
            libpcre3-dev \
            libev-dev \
            libavl-dev \
            libprotobuf-dev \
            libprotobuf-c-dev \
            libprotoc-dev \
            protobuf-compiler \
            protobuf-c-compiler \
            protobuf-compiler-grpc \
            libgrpc-dev \
            libgrpc++-dev \
            libboost-dev \
            libboost-program-options-dev \
            libboost-filesystem-dev \
            libboost-system-dev \
            libboost-thread-dev \
            libtool \
            pkg-config \
            thrift-compiler \
            python3 \
            python-is-python3 \
            python3-dev \
            python3-pip \
            python3-setuptools
RUN apt-get update \
    && apt-get install -y --no-install-recommends $BUILDER_DEPS \
    && ldconfig \
    && mkdir -p /output/usr/local
WORKDIR /

# Build libyang
FROM base-builder AS libyang
RUN git clone https://github.com/CESNET/libyang.git \
  && cd libyang \
  && git checkout 13b20f9
WORKDIR /libyang
RUN mkdir build && cd build \
  && cmake .. \
  && make DESTDIR=/output install

# Build sysrepo
FROM base-builder AS sysrepo
COPY --from=libyang /output/usr/local /usr/local/
RUN ldconfig  
RUN git clone https://github.com/sysrepo/sysrepo.git \
  && cd sysrepo \
  && git checkout cbd2a30
WORKDIR /sysrepo
# CALL_TARGET_BINS_DIRECTLY=Off is needed here because of the use of DESTDIR
# Without it sysrepoctl is executed at install time and assumes YANG files are
# under /etc/sysrepo/yang
RUN mkdir build \
  && cd build \
  && cmake -DCMAKE_BUILD_TYPE=Release -DBUILD_EXAMPLES=Off -DCALL_TARGET_BINS_DIRECTLY=Off .. \
  && make DESTDIR=/output install

# Build PI
FROM base-builder AS pi
COPY --from=libyang /output/usr/local /usr/local/
COPY --from=sysrepo /output/usr/local /usr/local/
RUN ldconfig  
RUN git clone https://github.com/p4lang/PI.git \
  && cd PI \
  && git checkout 5689c91 \
  && git submodule update --init
WORKDIR /PI
RUN ./autogen.sh && \
    ./configure --enable-Werror --without-bmv2 --without-internal-rpc --without-cli --with-proto --with-sysrepo && \
    make && \
    make DESTDIR=/output install-strip

# Build bmv2 with PI and grpc support
FROM base-builder AS bmv2
COPY --from=libyang /output/usr/local /usr/local/
COPY --from=sysrepo /output/usr/local /usr/local/
COPY --from=pi /output/usr/local /usr/local/
RUN ldconfig  
RUN git clone https://github.com/p4lang/behavioral-model.git \
  && cd behavioral-model \
  && git checkout 0456d64
WORKDIR /behavioral-model
RUN ./autogen.sh && \
  ./configure  --with-thrift --with-pi --with-pdfixed --with-stress-tests --enable-debugger --enable-Werror && \
    make && \
    make DESTDIR=/output install-strip

FROM debian:bookworm-slim
LABEL maintainer="Mickaël Royer <mickael.royer@enac.fr>"
LABEL description="Docker image for gonetem that includes bmv2 P4 software switch."
ENV DEBIAN_FRONTEND=noninteractive
ENV RUN_DEPS libboost-program-options1.74.0 \
    libboost-system1.74.0 \
    libboost-filesystem1.74.0 \
    libboost-thread1.74.0 \
    libgmp10 \
    libpcap0.8 \
    libnanomsg5 \
    libthrift-0.17.0 \
    libprotobuf32  \
    libprotobuf-c1 \
    libgrpc29 \
    libgrpc++1.51 \
    libpcre3 \
    libev4 \
    libavl1 \
    python3 \
    python-is-python3 \
    python3-pyroute2 \
    python3-six \
    python3-thrift \
    tcpdump \
    htop \
    ethtool \
    procps \
    psmisc \
    net-tools \
    iproute2 \
    iptables \
    vim
COPY --from=libyang /output/usr/local /usr/local/
COPY --from=sysrepo /output/usr/local /usr/local/
COPY --from=sysrepo /output/etc /etc/
COPY --from=PI /output/usr/local /usr/local/
COPY --from=bmv2 /output/usr/local /usr/local/
RUN apt-get update \
    && apt-get install -y --no-install-recommends $RUN_DEPS \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir /models && mkdir /config
COPY ./conf/basic.json /models/default.json
WORKDIR /models

COPY ./scripts/launch-sw.py /usr/local/bin/
RUN chmod +x /usr/local/bin/launch-sw.py

COPY ./scripts/iinit.py /usr/bin/
RUN chmod +x /usr/bin/iinit.py
CMD ["/usr/bin/iinit.py"]

