#!/usr/bin/python3

import subprocess
import argparse
import shlex
import logging
import signal
from pyroute2 import NDB
from daemonize import Daemonize


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

    # init logger
    LEVEL = logging.INFO

    logger = logging.getLogger(__name__)
    logger.setLevel(LEVEL)
    logger.propagate = False
    fh = logging.FileHandler(args.log, "w")
    fh.setLevel(LEVEL)
    logger.addHandler(fh)
    keep_fds = [fh.stream.fileno()]

    def launch_ss(): 
        interfaces = get_interface_list()
        cmd = "/usr/local/bin/simple_switch_grpc"
        for idx, ifname in enumerate(interfaces):
            cmd += f" -i {idx}@{ifname}"
        cmd += " /models/default.json"

        logger.info(f"Running command: {cmd}")

        with subprocess.Popen(shlex.split(cmd), stdout=subprocess.PIPE, stderr=subprocess.STDOUT) as proc:
            for line in proc.stdout:
                logger.info(line.decode("utf-8"))


    daemon = Daemonize(app="simple_switch", pid=args.pid, action=launch_ss, keep_fds=keep_fds)
    daemon.start()





