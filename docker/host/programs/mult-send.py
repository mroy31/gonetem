#!/usr/bin/env python3

import socket
import argparse
import logging
import time
import signal

PORT = 5222
TTL = 64
RUNNING = True

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Multicast sending program')
    parser.add_argument(
        "-g", "--group", type=str, dest="group",
        metavar="IP_ADDR", default="239.1.1.1",
        help="Multicast IP address destination")
    parser.add_argument(
        "-i", "--interval", type=float, dest="interval",
        metavar="INTERVAL", default=1.0,
        help="Interval between 2 packets")
    args = parser.parse_args()
    
    log_format = '%(levelname)s: %(message)s'
    logging.basicConfig(format=log_format, level=logging.INFO)

    sock = socket.socket(socket.AF_INET,
                        socket.SOCK_DGRAM,
                        socket.IPPROTO_UDP)
    sock.setsockopt(socket.IPPROTO_IP,
                    socket.IP_MULTICAST_TTL,
                    TTL)
    
    def handler(_signum, _frame):
        global RUNNING

        logging.info("Stop sending...")
        RUNNING = False

    signal.signal(signal.SIGINT, handler)
    signal.signal(signal.SIGTERM, handler)

    logging.info(f"Send multicast packets to group {args.group}")
    counter = 1
    while RUNNING:
        logging.info(f"Send packet {counter}")
        sock.sendto(f"Multicat Packet {counter}".encode('utf-8'), (args.group, PORT))
        counter += 1
        time.sleep(args.interval)
    
    sock.close()
