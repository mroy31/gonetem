#!/usr/bin/python3

import sys
import argparse
import shlex
import subprocess
import re
from cmd2 import Cmd
from cmd2 import Cmd2ArgumentParser, with_argparser


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


def parse_bonding_infos(name: str, infos: str) -> str:
    result = ""
    members = []

    for line in infos.split("\n"):
        if line.startswith("bond_mode: "):
            mode = line.replace("bond_mode: ", "")
            result += f"Mode: {mode}\n"
        elif line.startswith("member "):
            groups = re.match(r"^member\s+(\S+): (\S+)$", line)
            if groups is None:
                continue
            ifname = groups[1].split(".")[1]
            members.append(f"{ifname}: {groups[2]}")

    result += "Members:\n"
    for member in members:
        result += "\t" + member + "\n"
    return result


class OvsConsole(Cmd):
    intro = "Welcome to switch console. Type help or ? to list commands."

    def __init__(self, sw_name: str):
        self.prompt = f"[{sw_name}]>"
        super(OvsConsole, self).__init__(allow_cli_args=False)
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

    def do_exit(self, _: str):
        """Alias for quit"""
        return True

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

    add_argparser = Cmd2ArgumentParser()
    add_argparser.add_argument('name', help='name of the bonding')
    add_argparser.add_argument('ifaces', nargs="+", help='name of the bonding', type=int)

    @with_argparser(add_argparser)
    def do_bonding(self, opts):
        """Create a bond interface (with LACP active) and attach ifaces"""
        if len(opts.ifaces) < 2:
            self.perror("A least 2 interfaces is required")
            return

        try:
            ifaces = " ".join([f"{self.sw_name}.{i}" for i in opts.ifaces])
            run_command(f"ovs-vsctl add-bond {self.sw_name} {self.sw_name}.{opts.name} {ifaces} lacp=active")
        except ConsoleError as err:
            self.perror(f"Unable to create bonding {opts.name}: {err}")

    del_argparser = Cmd2ArgumentParser()
    del_argparser.add_argument('name', help='name of the bonding')

    @with_argparser(del_argparser)
    def do_no_bonding(self, opts):
        """Delete a bond interface"""
        try:
            run_command(f"ovs-vsctl del-port {self.sw_name} {self.sw_name}.{opts.name}")
        except ConsoleError as err:
            self.perror(f"Unable to delete bonding {opts.name}: {err}")

    show_argparser = Cmd2ArgumentParser()
    show_argparser.add_argument('name', help='name of the bonding', default=None)

    @with_argparser(show_argparser)
    def do_bonding_show(self, opts):
        """Display information on a bond interface"""
        if opts.name is None:
            self.perror("You need to specify the name of the bond interface")
            return

        try:
            infos = run_command(
                f"ovs-appctl bond/show {self.sw_name}.{opts.name}",
                check_output=True)
            self.poutput(parse_bonding_infos(self.sw_name, infos))
        except ConsoleError as err:
            self.perror(f"Unable to get infos on bonding interface {opts.name}: {err}")


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
