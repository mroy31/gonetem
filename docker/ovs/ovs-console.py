#!/usr/bin/python3

import sys
import argparse
import shlex
import subprocess
import re
from cmd2 import Cmd


class ConsoleError(Exception):
    pass


def run_command(cmd_line: str, check_output: bool = False, shell: bool = False) -> str:
    args = shlex.split(cmd_line)
    if check_output:
        try:
            result = subprocess.check_output(args, shell=shell)
        except subprocess.CalledProcessError as err:
            raise ConsoleError("Error in command '{}': {}\n".format(cmd_line, err))
        except FileNotFoundError as err:
            raise ConsoleError("Error in command '{}': {}\n".format(cmd_line, err))
        return result.decode("utf-8").strip("\n")
    else:
        ret = subprocess.call(args, shell=shell)
        if ret != 0:
            raise ConsoleError(f"Error in command '{cmd_line}' retcode != 0")

    return ""


def is_sw_exist(name: str) -> bool:
    args = shlex.split("ovs-vsctl br-exists {}".format(name))
    return subprocess.run(args).returncode != 2


def list_sw_ports(name: str) -> str:
    return run_command(f"ovs-vsctl list-ports {name}", check_output=True)


class OvsConsole(Cmd):
    intro = "Welcome to switch console. Type help or ? to list commands."

    def __init__(self, sw_name: str):
        self.prompt = f"[{sw_name}]>"
        super(OvsConsole, self).__init__(allow_cli_args=False, use_ipython=False)
        # disable some commands
        disable_commmands = [
            "edit", "py", "set", "run_pyscript", "run_script",
            "shortcuts", "shell", "macro", "alias"]
        for cmd_name in disable_commmands:
            if hasattr(Cmd, "do_"+cmd_name):
                delattr(Cmd, "do_"+cmd_name)

        self.sw_name = sw_name

    def emptyline(self):
        # do nothing when an empty line is entered
        pass

    def do_vlan_show(self, _: str):
        """Show actual configuration"""
        ports = list_sw_ports(self.sw_name).split("\n")
        for port in ports:
            if not port.startswith(self.sw_name+"."):
                continue
            self.poutput(f"Port {port.split('.')[1]}")

            try:
                # get vlan
                tag = run_command(f"ovs-vsctl get port {port} tag", check_output=True)
                if tag == "[]":
                    tag = "0"
                self.poutput(f"  VLAN Access: {tag}")
                # get trunks
                trunks = run_command(f"ovs-vsctl get port {port} trunks", check_output=True)
                if trunks != "[]":
                    self.poutput(f"  VLAN Trunk: {trunks.strip('[]')}")
            except ConsoleError as err:
                self.perror("Unable to get port info: {}".format(err))

    def do_vlan_access(self, statement: str):
        """Add a port to a VLAN in access mode"""
        groups = re.match(r"port\s+(\d+)\s+vlan\s+(\d+)$", statement)
        if groups is None:
            self.perror("This command has to follow this syntax: port <port-number> vlan <vlan-id>")
            return

        port, vlan = groups[1], groups[2]
        try:
            run_command(f"ovs-vsctl set port {self.sw_name}.{port} tag={vlan}")
        except ConsoleError as err:
            self.perror("Unable add port {} to vlan {}: {}".format(port, vlan, err))

    def do_no_vlan_access(self, statement: str):
        """Remove a port to a VLAN in access mode"""
        groups = re.match(r"port\s+(\d+)\s+vlan\s+(\d+)$", statement)
        if groups is None:
            self.perror("This command has to follow this syntax: port <port-number> vlan <vlan-id>")
            return

        port, vlan = groups[1], groups[2]
        try:
            run_command(f"ovs-vsctl remove port {self.sw_name}.{port} tag {vlan}")
        except ConsoleError as err:
            self.perror("Unable remove port {} to vlan {}: {}".format(port, vlan, err))

    def do_vlan_trunks(self, statement: str):
        """Add a port to vlans in trunk mode"""
        groups = re.match(r"port\s+(\d+)\s+vlans\s+([\d|,]+)$", statement)
        if groups is None:
            self.perror("This command has to follow this syntax: port <port-number> vlans <vlan-id1>,<vlan-id2>")
            return

        port, trunks = groups[1], groups[2]
        try:
            run_command(
                f"ovs-vsctl set port {self.sw_name}.{port} trunks={trunks}"
            )
        except ConsoleError as err:
            self.perror(
                "Unable add port {} to trunks {}: {}".format(port, trunks, err)
            )

    def do_no_vlan_trunks(self, statement: str):
        """Remove a port to vlans in trunk mode"""
        groups = re.match(r"port\s+(\d+)\s+vlans\s+([\d|,]+)$", statement)
        if groups is None:
            self.perror("This command has to follow this syntax: port <port-number> vlans <vlan-id1>,<vlan-id2>")
            return

        port, trunks = groups[1], groups[2]
        try:
            run_command(
                f"ovs-vsctl remove port {self.sw_name}.{port} trunks {trunks}"
            )
        except ConsoleError as err:
            self.perror(
                "Unable remove port {} to trunks {}: {}".format(port, trunks, err)
            )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Simple python console for openvswitch')
    parser.add_argument(
        'sw', metavar='SW_NAME', type=str, nargs="?",
        default=None, help='Name of the switch')
    args = parser.parse_args()

    if args.sw is None:
        sys.exit("You must enter an ovs switch name")
    elif not is_sw_exist(args.sw):
        sys.exit(f"Ovs switch '{args.sw}' not found")

    OvsConsole(args.sw).cmdloop()
