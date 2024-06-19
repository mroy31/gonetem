#!/usr/bin/python3

import time
import sys
import json
import os.path
import subprocess
import argparse
import shlex
from typing import TypedDict

class BondingInfosT(TypedDict):
    members: list[str]
    name: str


class RunError(Exception):
    pass


def run_command(cmd_line: str, check_output: bool = False, shell: bool = False) -> str:
    args = shlex.split(cmd_line)
    if check_output:
        try:
            result = subprocess.check_output(args, shell=shell)
        except subprocess.CalledProcessError as err:
            raise RunError("Error in command '{}': {}\n".format(cmd_line, err))
        except FileNotFoundError as err:
            raise RunError("Error in command '{}': {}\n".format(cmd_line, err))
        else:
            return result.decode("utf-8").strip("\n")
    else:
        ret = subprocess.call(args, shell=shell)
        if ret != 0:
            raise RunError(f"Error command '{cmd_line}' retcode != 0")

    return ""


def is_sw_exist(name: str) -> bool:
    args = shlex.split("ovs-vsctl br-exists {}".format(name))
    return subprocess.run(args).returncode != 2


def list_sw_ports(name: str) -> str:
    ports = run_command(f"ovs-vsctl list-ports {name}", check_output=True)
    return [p for p in ports.split("\n") if p != '']


def list_bond_ports(sw_name: str) -> list[BondingInfosT]:
    infos: list[BondingInfosT] = []

    bond_ifaces = run_command(f"ovs-appctl bond/list", check_output=True)
    for line in bond_ifaces.splitlines():
        if line.startswith(sw_name):
            data = line.split("\t")
            members = [i.strip() for i in data[3].split(",")]
            infos.append({
                'name': data[0],
                'members': members,
            })
    
    return infos

def port_is_bonding(bonds: list[BondingInfosT], if_name: str) -> BondingInfosT:
    for bond in bonds:
        if bond["name"] == if_name:
            return bond

    return None


def load_ovs_config(sw_name: str, conf: str):
    with open(conf) as f_hd:
        last_error = ""
        ovs_config = json.load(f_hd)

        attempt = 0
        while attempt < 10:
            try:
                for p_config in ovs_config:
                    if "bonding" in p_config:
                        for iface in p_config["bonding"]["members"]:
                            run_command(f"ovs-vsctl del-port {sw_name} {iface}")

                        ifaces = " ".join(p_config["bonding"]["members"])
                        run_command(f"ovs-vsctl add-bond {sw_name} {p_config['name']} {ifaces} lacp=active")

                    if "tag" in p_config:
                        run_command("ovs-vsctl set port {} tag={}".format(p_config["name"], p_config["tag"]))

                    if "trunks" in p_config:
                        run_command("ovs-vsctl set port {} trunks={}".format(p_config["name"], p_config["trunks"]))
            except RunError as err:
                attempt += 1
                last_error = str(err)
                time.sleep(0.1)
                continue
            else:
                return

        # All attempts fail, exit with error
        sys.exit("Unable to load config: {}".format(last_error))


def save_ovs_config(sw_name: str, conf: str):
    config = []
    try:
        bonds = list_bond_ports(sw_name)

        ports = list_sw_ports(sw_name)
        for port in ports:
            # save vlan config
            p_config = {"name": port}
            tag = run_command(f"ovs-vsctl get port {port} tag", check_output=True)
            if tag != "[]":
                p_config["tag"] = tag
            trunks = run_command(f"ovs-vsctl get port {port} trunks", check_output=True)
            if trunks != "[]":
                p_config["trunks"] = trunks.strip("[]").replace(" ", "")
            
            # save bonding config
            bonding_conf = port_is_bonding(bonds, port)
            if bonding_conf is not None:
                p_config["bonding"] = {
                    "members": bonding_conf["members"]
                }

            config.append(p_config)

    except RunError as err:
        sys.exit("Unable to save config: {}".format(err))

    # remove old file if exist
    os.path.isfile(conf) and os.remove(conf)

    with open(conf, "w") as f_hd:
        f_hd.write(json.dumps(config, sort_keys=True, indent=4))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Simple python console for openvswitch')
    parser.add_argument(
        "-c", "--conf", type=str, dest="conf",
        metavar="FILE", default=None,
        help="Conf file to load/save")
    parser.add_argument(
        "-a", "--action", type=str, dest="action",
        metavar="save|load", default=None,
        help="Action: load or save")
    parser.add_argument(
        'sw', metavar='SW_NAME', type=str, nargs="?",
        default=None, help='Name of the switch')
    args = parser.parse_args()

    if args.conf is None:
        sys.exit("Conf file is required")
    if args.action not in ("load", "save"):
        sys.exit("Required action : load or save")

    if args.action == "load":
        if not os.path.isfile(args.conf):
            sys.exit(f"{args.conf} file does not exist")
        try:
            load_ovs_config(args.sw, args.conf)
        except Exception as err:
            sys.exit("Unable to load configuration: {}".format(err))
    elif args.action == "save":
        try:
            save_ovs_config(args.sw, args.conf)
        except Exception as err:
            sys.exit("Unable to load configuration: {}".format(err))
