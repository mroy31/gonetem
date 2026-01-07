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
import ipaddress
from optparse import OptionParser
from pyroute2 import NDB
import socket
from pyroute2.netlink.rtnl import ifaddrmsg

IFA_F_TEMPORARY = ifaddrmsg.IFA_F_TEMPORARY
IFA_F_MANAGETEMPADDR = ifaddrmsg.IFA_F_MANAGETEMPADDR
IFA_F_PERMANENT = ifaddrmsg.IFA_F_PERMANENT


def get_addr_kind(addr):
    f =  addr.flags

    if (
        f & IFA_F_TEMPORARY
        or f & IFA_F_MANAGETEMPADDR
    ):
        return "SLAAC"

    if f & IFA_F_PERMANENT:
        return "PERMANENT"

    return "DYNAMIC"


def record_addr(addr):
    if (
        addr.family == socket.AF_INET6
        and addr.scope == 0
    ):
        return {
                "address": f"{addr.address}/{addr.prefixlen}",
                "version": 6,
                "kind": get_addr_kind(addr)
            }

    if (
        addr.family == socket.AF_INET
        and addr.scope == 0
    ):
        return {
                "address": f"{addr.address}/{addr.prefixlen}",
                "version": 4,
                "kind": get_addr_kind(addr)
            }

    return None



def is_interface_exists(ndb, if_name):
    for k in ndb.interfaces:
        if_obj = ndb.interfaces[k]
        if if_name == if_obj["ifname"]:
            return True

    return False


def is_interface_exists_by_idx(ndb, if_idx):
    for k in ndb.interfaces:
        if_obj = ndb.interfaces[k]
        if if_idx == if_obj["index"]:
            return True

    return False


def get_interface_byindex(ndb, target_idx):
    for k in ndb.interfaces:
        if_obj = ndb.interfaces[k]
        if target_idx == if_obj["index"]:
            return if_obj["ifname"]
    
    return None


def load_interface_addrs(ndb, if_name, ip_addrs):
    try:
        i = ndb.interfaces[if_name].set('state', 'up').commit()
    except KeyError:
        return  # interface not found

    for addr_obj in ip_addrs:
        address = addr_obj
        is_permanent = True
        if type(addr_obj) is dict:
            address = addr_obj["address"]
            is_permanent = addr_obj["kind"] == "PERMANENT"

        if is_permanent:
            try:
                i = i.add_ip(address).commit()
            except Exception as ex:
                print("Unable to load IP address {} to interface {} -> {}".format(address, if_name, ex))


def create_interfaces(ndb, net_config):
    # create bonding interfaces
    for bond_name in net_config["bondings"]:
        bond_conf = net_config["bondings"][bond_name]

        try:
            bond_if = ndb.interfaces.create(
                ifname=bond_name,
                kind='bond', 
                bond_mode=bond_conf["mode"]).commit()
        except Exception as ex:
            print(f"Unable to create bonding interface {bond_name} -> {ex}")
            continue

        try:
            for slave_if in bond_conf["slaves"]:
                if not is_interface_exists(ndb, slave_if):
                    print(f"Interface {slave_if} does not exist")
                    continue
                ndb.interfaces[slave_if].set('state', 'down').commit()
                bond_if.add_port(slave_if).commit()
        except Exception as ex:
            print(f"Unable to configure bonding {bond_name} -> {ex}")
            continue

    # create vlan configurations
    for vlan_name in net_config["vlans"]:
        vlan_conf = net_config["vlans"][vlan_name]

        vlan_link = vlan_conf["link"]
        if type(vlan_conf["link"]) is int:
            vlan_link = get_interface_byindex(ndb, vlan_conf["link"])

        if vlan_link is None or not is_interface_exists(ndb, vlan_link):
            print("Interface " + str(vlan_conf["link"]) + " does not exist")
            continue

        try:
            ndb.interfaces.create(
                ifname=vlan_name,       
                kind='vlan',            
                link=vlan_link,
                vlan_id=vlan_conf["vlan_id"]
            ).commit()
        except Exception as ex:
            print(f"Unable to create vlan interface {vlan_name} -> {ex}")
            continue


def load_net_config(f_path):
    with open(f_path) as f_hd:
        net_config = json.load(f_hd)
        if "bondings" not in net_config:
            net_config["bondings"] = {}
        if "vlans" not in net_config:
            net_config["vlans"] = {}

        with NDB(sources=[{'target': 'localhost'}]) as ndb:
            create_interfaces(ndb, net_config)

            # load bonding IP addresses
            for bond_name in net_config["bondings"]:
                bond_conf = net_config["bondings"][bond_name]
                load_interface_addrs(ndb, bond_name, bond_conf["addresses"])

            # load VLAN IP addresses
            for vlan_name in net_config["vlans"]:
                vlan_conf = net_config["vlans"][vlan_name]
                if "addresses" not in vlan_conf:
                    continue
                load_interface_addrs(ndb, vlan_name, vlan_conf["addresses"])

            # load other interface addresses
            for ifname in net_config["interfaces"]:
                load_interface_addrs(ndb, ifname, net_config["interfaces"][ifname])

            # configure routes
            for route in net_config["routes"]:
                if route["gateway"] is not None:
                    try:
                        ndb.routes.create(**route).commit()
                    except Exception as ex:
                        print("Unable to load route {} via {} -> {}".format(route["dst"], route["gateway"], ex))


def save_net_config(f_path, all_if):
    net_config = {"interfaces": {}, "routes": [], "bondings": {}, "vlans": {}}
    with NDB(sources=[{'target': 'localhost'}]) as ndb: 
        def format_addresses(if_obj):
            return list(filter(
                lambda a: a is not None,
                [record_addr(a) for a in if_obj.ipaddr]
            ))

        # record interfaces config
        for k in ndb.interfaces:
            if_obj = ndb.interfaces[k]
            if_name = if_obj["ifname"]

            if if_obj["kind"] == "bond":
                bond_config = net_config["bondings"].get(if_name, { "slaves": [] })
                bond_config["mode"] = if_obj["bond_mode"]

                bond_config["addresses"] = format_addresses(if_obj)
                net_config["bondings"][if_name] = bond_config
                continue

            if if_obj["kind"] == "vlan":
                net_config["vlans"][if_name] = {
                    "link": get_interface_byindex(ndb, if_obj["link"]),
                    "vlan_id": if_obj["vlan_id"],
                    "addresses": format_addresses(if_obj)
                }

            if if_obj["slave_kind"] == "bond":
                master = get_interface_byindex(ndb, if_obj["master"])
                if master is not None:
                    bond_config = net_config["bondings"].get(master, { "slaves": [] })
                    bond_config["slaves"].append(if_name)
                    net_config["bondings"][master] = bond_config

            if not all_if and not if_name.startswith("eth"):
                continue

            net_config["interfaces"][if_name] = format_addresses(if_obj)

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
