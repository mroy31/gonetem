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
from pyroute2 import IPDB


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


def load_net_config(f_path):
    with open(f_path) as f_hd:
        net_config = json.load(f_hd)
        with IPDB() as ipdb:
            # configure ip addresses
            for ifname in net_config["interfaces"]:
                if ifname not in ipdb.interfaces:
                    continue
                with ipdb.interfaces[ifname] as i:
                    i.up()
                    for address in net_config["interfaces"][ifname]:
                        try:
                            i.add_ip(address)
                        except Exception as ex:
                            print("Unable to load IP address {} to interface {} -> {}".format(address, ifname, ex))
            # configure routes
            for route in net_config["routes"]:
                if route["gateway"] is not None:
                    try:
                        ipdb.routes.add(route).commit()
                    except Exception as ex:
                        print("Unable to load route {} via {} -> {}".format(route["dst"], route["gateway"], ex))


def save_net_config(f_path, all_if):
    def is_recordable(addr, if_name):
        ip_address = ipaddress.ip_address(addr)
        if ip_address.version == 6:
            if addr.startswith("fe80"):
                return False

        return True

    def fmt_addr(addr_conf):
        return "%s/%s" % addr_conf

    net_config = {"interfaces": {}, "routes": []}
    with IPDB() as ipdb:
        # record ip addresses
        for if_name in ipdb.interfaces:
            if isinstance(if_name, int):
                continue
            if not all_if and not if_name.startswith("eth"):
                continue
            addresses = ipdb.interfaces[if_name]["ipaddr"]

            net_config["interfaces"][if_name] = [
                fmt_addr(a) for a in addresses if is_recordable(a[0], if_name)
            ]
        # record route
        net_config["routes"] = [
            {
                "dst": route["dst"],
                "gateway": route["gateway"],
                "family": route["family"],
            }
            for route in ipdb.routes
            if route["gateway"] is not None and not route["gateway"].startswith("fe80")
        ]

    # remove old file if exist
    os.path.isfile(f_path) and os.remove(f_path)

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
