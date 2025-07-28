#!/usr/bin/python3
# gonetem: network emulator
# Copyright (C) 2015-2017 Mickael Royer <mickael.royer@recherche.enac.fr>
#
# This program is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 2 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License along
# with this program; if not, write to the Free Software Foundation, Inc.,
# 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.

import sys
import json
import os.path
import subprocess
import ipaddress
from optparse import OptionParser
from pyroute2 import NDB


def is_ipv6_autoconf(if_name):
    proc_prefix = "/proc/sys/net/ipv6/conf/"
    ret = subprocess.run(["cat", proc_prefix + "all/autoconf"], capture_output=True)
    if ret.output == "1":
        return True

    ret = subprocess.run(
        ["cat", proc_prefix + if_name + "/autoconf"], capture_output=True
    )
    if ret.output == "1":
        return True

    return False


def get_interface_byindex(ndb, target_idx):
    for k in ndb.interfaces:
        if_obj = ndb.interfaces[k]
        if target_idx == if_obj["index"]:
            return if_obj["ifname"]
    
    return None


def load_interface_config(ndb, if_name, ip_addrs):
    try:
        i = ndb.interfaces[if_name].set('state', 'up').commit()
    except KeyError:
        return  # interface not found

    for address in ip_addrs:
        try:
            i = i.add_ip(address).commit()
        except Exception as ex:
            print("Unable to load IP address {} to interface {} -> {}".format(address, if_name, ex))


def load_net_config(f_path):
    with open(f_path) as f_hd:
        net_config = json.load(f_hd)
        if "bondings" not in net_config:
            net_config["bondings"] = {}
        if "vlans" not in net_config:
            net_config["vlans"] = {}

        with NDB(sources=[{'target': 'localhost'}]) as ndb:
            # load vlan configurations
            for vlan_name in net_config["vlans"]:
                vlan_conf = net_config["vlans"][vlan_name]
                try:
                    ndb.interfaces.create(
                       ifname=vlan_name,       
                       kind='vlan',            
                       link=vlan_conf["link"],
                       vlan_id=vlan_conf["vlan_id"]
                    ).commit()
                except Exception as ex:
                    print(f"Unable to create vlan interface {vlan_name} -> {ex}")
                    continue

            # load interface configurations
            for ifname in net_config["interfaces"]:
                load_interface_config(ndb, ifname, net_config["interfaces"][ifname])

            # load bonding configurations
            for bond_name in net_config["bondings"]:
                try:
                    bond_if = ndb.interfaces.create(ifname=bond_name, kind='bond').commit()
                except Exception as ex:
                    print(f"Unable to create bonding interface {bond_name} -> {ex}")
                    continue

                bond_conf = net_config["bondings"][bond_name]
                try:
                    for slave_if in bond_conf["slaves"]:
                        ndb.interfaces[slave_if].set('state', 'down').commit()
                        bond_if.add_port(slave_if).commit()
                    bond_if.set("bond_mode", bond_conf["mode"]).commit()
                except Exception as ex:
                    print(f"Unable to configure bonding {bond_name} -> {ex}")
                    continue
                load_interface_config(ndb, bond_name, bond_conf["addresses"])

            # configure routes
            for route in net_config["routes"]:
                if route["gateway"] is not None:
                    try:
                        ndb.routes.create(**route).commit()
                    except Exception as ex:
                        print("Unable to load route {} via {} -> {}".format(route["dst"], route["gateway"], ex))


def save_net_config(f_path, all_if):
    def is_recordable(addr):
        ip_address = ipaddress.ip_address(addr)
        if ip_address.version == 6:
            if addr.startswith("fe80"):
                return False

        return True

    def fmt_addr(addr_conf):
        return f"{addr_conf['address']}/{addr_conf['prefixlen']}"

    net_config = {"interfaces": {}, "routes": [], "bondings": {}, "vlans": {}}
    with NDB(sources=[{'target': 'localhost'}]) as ndb: 
        # record interfaces config
        for k in ndb.interfaces:
            if_obj = ndb.interfaces[k]
            if_name = if_obj["ifname"]

            if if_obj["kind"] == "bond":
                bond_config = net_config["bondings"].get(if_name, { "slaves": [] })
                bond_config["mode"] = if_obj["bond_mode"]

                addresses = if_obj.ipaddr
                bond_config["addresses"] = [
                    fmt_addr(addresses[a]) for a in addresses if is_recordable(addresses[a]["address"])
                ]

                net_config["bondings"][if_name] = bond_config
                continue

            if if_obj["kind"] == "vlan":
                net_config["vlans"][if_name] = {
                    "link": if_obj["link"],
                    "vlan_id": if_obj["vlan_id"]
                }

            if if_obj["slave_kind"] == "bond":
                master = get_interface_byindex(ndb, if_obj["master"])
                if master is not None:
                    bond_config = net_config["bondings"].get(master, { "slaves": [] })
                    bond_config["slaves"].append(if_name)
                    net_config["bondings"][master] = bond_config

            if not all_if and not if_name.startswith("eth"):
                continue

            addresses = if_obj.ipaddr
            net_config["interfaces"][if_name] = [
                fmt_addr(addresses[a]) for a in addresses if is_recordable(addresses[a]["address"])
            ]

        # record route
        for route in ndb.routes:
            if route["gateway"] is None or route["gateway"].startswith("fe80"):
                continue

            dst = f"{route['dst']}/{route['dst_len']}"
            if dst == "/0": ## default route
                dst = "default"

            net_config["routes"].append({
                "dst": dst,
                "gateway": route["gateway"],
                "family": route["family"],
            })

    # remove old file if it exists
    if os.path.isfile(f_path):
        os.remove(f_path)

    with open(f_path, "w") as f_hd:
        f_hd.write(json.dumps(net_config, sort_keys=True, indent=4))


if __name__ == "__main__":
    usage = "usage: %prog [options] <network file>"
    parser = OptionParser(usage=usage)
    parser.add_option(
        "-s", "--save", action="store_true", dest="save", help="save network config"
    )
    parser.add_option(
        "-l", "--load", action="store_true", dest="load", help="load network config"
    )
    parser.add_option(
        "-a",
        "--all",
        action="store_true",
        dest="all",
        help="save config for all interfaces",
    )
    (options, args) = parser.parse_args()

    if len(args) != 1:
        sys.exit("You have to specify a network file")
    f_path = args[0]
    if options.load:
        if not os.path.isfile(f_path):
            sys.exit("%s file does not exist" % f_path)
        load_net_config(f_path)
    elif options.save:
        try:
            save_net_config(f_path, options.all)
        except Exception as ex:
            sys.exit("Unable to save net config: {}".format(ex))
