#!/usr/bin/python3

import sys
import os
import re
import logging
import argparse
import tempfile
import zipfile
import shutil
import tarfile
from configobj import ConfigObj
import yaml


def open_pnet_project(prj_path):
    tmp_folder = tempfile.mkdtemp(prefix="pnet")
    with zipfile.ZipFile(prj_path) as prj_zip:
        prj_zip.extractall(path=tmp_folder)

    return tmp_folder


def copy_config_files(pnet_path, gnet_path):
    c_path = os.path.join(pnet_path, "configs")
    if os.path.isdir(c_path):
        conf_files = [
            f for f in os.listdir(c_path)
            if os.path.isfile(os.path.join(c_path, f))
        ]

        for f in conf_files:
            shutil.copy(
                os.path.join(c_path, f),
                os.path.join(gnet_path, "configs"),
            )


def create_gnet_network(pnet_network):
    network = {"nodes": {}, "links": [], "bridges": {}}
    sw_indexes = {}

    def type_translation(pnet_type):
        if pnet_type == "docker.frr":
            return "docker.router"
        return pnet_type

    def get_sw_index(sw):
        if sw not in sw_indexes:
            sw_indexes[sw] = -1
        sw_indexes[sw] += 1

        return sw_indexes[sw]

    def is_peer_exist(peer):
        for lk in network["links"]:
            if lk["peer1"] == peer or lk["peer2"] == peer:
                return True
        return False

    if "bridges" in pnet_network:
        for br in pnet_network["bridges"]:
            b_config = pnet_network["bridges"][br]
            network["bridges"][br] = {
                "host": b_config["host_if"],
                "interfaces": []
            }

    if "switches" in pnet_network:
        for sw in pnet_network["switches"]:
            network["nodes"][sw] = {"type": "ovs"}

    for name in pnet_network["nodes"]:
        n_config = pnet_network["nodes"][name]
        node = {
            "type": type_translation(n_config["type"]),
        }

        if "ipv6" in n_config:
            node["ipv6"] = n_config.as_bool("ipv6")
        if "mpls" in n_config:
            node["mpls"] = n_config.as_bool("mpls")
        if "vrfs" in n_config:
            node["vrfs"] = n_config["vrfs"].split(";")

        network["nodes"][name] = node

        # create associated links
        for idx in range(n_config.as_int("if_numbers")):
            peer1 = "%s.%d" % (name, idx)
            if is_peer_exist(peer1):
                continue

            peer2 = n_config["if"+str(idx)]
            if peer2.startswith("sw"):
                (s_name,) = re.match(r"^sw\.(\w+)$", peer2).groups()
                peer2 = "%s.%d" % (s_name, get_sw_index(s_name))
                network["links"].append({"peer1": peer1, "peer2": peer2})
            elif peer2.startswith("br"):
                (b_name,) = re.match(r"^br\.(\w+)$", peer2).groups()
                for br in network["bridges"]:
                    if br.name == b_name:
                        br["interfaces"].append(peer1)
            elif not is_peer_exist(peer2):
                network["links"].append({"peer1": peer1, "peer2": peer2})

    return network


def create_gnet_archive(gnet_path, output_filename):
    with tarfile.open(output_filename, "w:gz") as tar:
        tar.add(gnet_path, arcname="")


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Network emulator console')
    parser.add_argument(
        'project', metavar='PRJ', type=str, nargs="?",
        default=None, help='Path for the pnet project')
    parser.add_argument(
        "-o", "--output", type=str, dest="output",
        metavar="FILE", default=None,
        help="Specify output gonetem project")
    args = parser.parse_args()

    log_format = '%(levelname)s: %(message)s'
    logging.basicConfig(format=log_format, level=logging.INFO)

    if args.output is None:
        sys.exit("gnet project path is required")

    _, ext = os.path.splitext(args.project)
    if ext.lower() != ".pnet":
        sys.exit("Project %s has wrong ext" % args.project)
    elif not os.path.isfile(args.project):
        sys.exit("Project %s does not exist" % args.project)

    pnet_dir = open_pnet_project(args.project)
    gnet_dir = tempfile.mkdtemp(prefix="gnet")
    os.mkdir(os.path.join(gnet_dir, "configs"))

    try:
        copy_config_files(pnet_dir, gnet_dir)

        pnet_network = ConfigObj(os.path.join(pnet_dir, "network.ini"))
        gnet_network = create_gnet_network(pnet_network)
        with open(os.path.join(gnet_dir, "network.yml"), "w") as hd:
            yaml.dump(gnet_network, stream=hd)

        create_gnet_archive(gnet_dir, args.output)
    finally:
        # clean temp folders
        shutil.rmtree(pnet_dir)
        shutil.rmtree(gnet_dir)
