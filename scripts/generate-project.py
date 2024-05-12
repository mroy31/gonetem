#!/usr/bin/env python3

import argparse
import logging
import os
import sys
import random
import tarfile

def generate_topology(node_count: int, launch_count: int) -> str:
    nodes_idx = list(range(0, node_count))
    launch_idx = random.choices(nodes_idx, k=launch_count)

    nodes = ""
    links = ""
    for idx in nodes_idx:
        launch = idx in launch_idx and "true" or "false"
        nodes += f"  sat{idx}:\n    type: docker.router\n    launch: {launch}\n"
        links += f"- peer1: ground.{idx}\n  peer2: sat{idx}.0\n"

    return f"""
nodes:
  ground:
    type: docker.host
{nodes}
links:
{links}
"""

def generate_project(node_count: int, launch_count: int, output: str) -> str:
    with open("network.yml", "w") as fd:
        fd.write(generate_topology(node_count, launch_count))    
    
    with tarfile.open(output, "w:gz") as tar:
        tar.add("network.yml")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Gonetem sat topolgy generator')
    parser.add_argument(
        "-o", "--output", type=str, dest="output",
        metavar="FILE", default=None,
        help="Output project")
    parser.add_argument(
        "-n", "--nodes", type=int, dest="nodes",
        metavar="NODES", default=10,
        help="Number of nodes in the network (default: 10)")
    parser.add_argument(
        "-l", "--launch", type=int, dest="launch",
        metavar="NUMBER", default=1,
        help="Number of nodes started when the network is launched (default: 1)")
    args = parser.parse_args()

    log_format = '%(levelname)s: %(message)s'
    logging.basicConfig(format=log_format, level=logging.INFO)

    if args.output is None:
        logging.error("You have to set an output file")
        sys.exit(1)

    if args.launch > args.nodes:
        logging.error("Launch arg is greater than nodes arg")
        sys.exit(1)
    
    generate_project(args.nodes, args.launch, args.output)