#!/usr/bin/env python3

import socket
import argparse
import logging
import signal
import struct

# WARNING: works only for linux
if not hasattr(socket, 'IP_UNBLOCK_SOURCE'):
    setattr(socket, 'IP_UNBLOCK_SOURCE', 37)
if not hasattr(socket, 'IP_BLOCK_SOURCE'):
    setattr(socket, 'IP_BLOCK_SOURCE', 38)
if not hasattr(socket, 'IP_ADD_SOURCE_MEMBERSHIP'):
    setattr(socket, 'IP_ADD_SOURCE_MEMBERSHIP', 39)
if not hasattr(socket, 'IP_DROP_SOURCE_MEMBERSHIP'):
    setattr(socket, 'IP_DROP_SOURCE_MEMBERSHIP', 40)


PORT = 5222
RUNNING = True

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Multicast receiving program')
    parser.add_argument(
        "-g", "--group", type=str, dest="group",
        metavar="IP_ADDR", default="239.1.1.1",
        help="Multicast IP address (239.1.1.1 by default)")
    parser.add_argument(
        "-s", "--source", type=str, dest="source",
        metavar="IP_ADDRESS", default=None,
        help="IP source to filter IGMP report (optionnal)")
    args = parser.parse_args()
    
    log_format = '%(levelname)s: %(message)s'
    logging.basicConfig(format=log_format, level=logging.INFO)

    sock = socket.socket(socket.AF_INET,
                        socket.SOCK_DGRAM,
                        socket.IPPROTO_UDP)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(('', PORT))

    if args.source is None:
        logging.info(f"Listen on multicast address {args.group}")
        mreq = struct.pack("4sl", socket.inet_aton(args.group), socket.INADDR_ANY)
        sock.setsockopt(socket.IPPROTO_IP, socket.IP_ADD_MEMBERSHIP, mreq)
    else:
        logging.info(f"Listen on multicast address {args.group} only for source {args.source}")
        mreq = struct.pack("=4sl4s", socket.inet_aton(args.group), socket.INADDR_ANY, socket.inet_aton(args.source))
        sock.setsockopt(socket.IPPROTO_IP, socket.IP_ADD_SOURCE_MEMBERSHIP, mreq)

    sock.settimeout(1.0)
    
    def handler(_signum, _frame):
        global RUNNING

        logging.info("Stop receiving...")
        RUNNING = False

    signal.signal(signal.SIGINT, handler)
    signal.signal(signal.SIGTERM, handler)

    while RUNNING:
        try:
            data = sock.recv(10240)
        except TimeoutError:
            continue
        logging.info(f"Receive packet {data}")
    
    sock.close()
