#!/usr/bin/python3

import subprocess
import argparse
import shlex
import logging
import os
import errno
import sys
from pyroute2 import NDB


def daemonize():
    # See http://www.erlenstar.demon.co.uk/unix/faq_toc.html#TOC16
    if os.fork():  # launch child and...
        os._exit(0)  # kill off parent
    os.setsid()
    if os.fork():  # launch child and...
        os._exit(0)  # kill off parent again.
    os.umask(0o77)
    null = os.open('/dev/null', os.O_RDWR)
    for i in range(3):
        try:
            os.dup2(null, i)
        except OSError as e:
            if e.errno != errno.EBADF:
                raise
    os.close(null)


def removePID(pidfile):
    if os.path.isfile(pidfile):
        try: os.remove(pidfile)
        except OSError as e:
            if e.errno == errno.EACCES or e.errno == errno.EPERM:
                sys.exit(f"Unable to remove pid file : {str(e)}")


def get_interface_list():
    interfaces = []
    with NDB(sources=[{'target': 'localhost'}]) as ndb: 
        for k in ndb.interfaces:
            if_obj = ndb.interfaces[k]
            if if_obj["ifname"].startswith("eth"):
                interfaces.append(if_obj["ifname"])
    
    return sorted(interfaces)



if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Launch bmv2 simple_swith')
    parser.add_argument(
        "-p", "--pid", type=str, dest="pid",
        metavar="PID", default="/var/run/bmv2-ss.pid",
        help="PID file")
    parser.add_argument(
        "-l", "--log", type=str, dest="log",
        metavar="FILE", default="/var/log/bmv2-ss.log",
        help="Log file")
    args = parser.parse_args()

    # daemonize
    removePID(args.pid)
    daemonize()

    # init logger
    LEVEL = logging.INFO

    logger = logging.getLogger(__name__)
    logger.setLevel(LEVEL)
    logger.propagate = False
    fh = logging.FileHandler(args.log, "w")
    fh.setLevel(LEVEL)
    logger.addHandler(fh)
    keep_fds = [fh.stream.fileno()]

    interfaces = get_interface_list()
    cmd = "/usr/local/bin/simple_switch_grpc"
    for idx, ifname in enumerate(interfaces):
        cmd += f" -i {idx}@{ifname}"
    cmd += " /models/default.json"

    logger.info(f"Running command: {cmd}")
    subprocess.run(
        shlex.split(cmd),
        stdout=fh.stream.fileno(),
        stderr=subprocess.STDOUT)







