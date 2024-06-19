#!/usr/bin/python3

import sys
import argparse
import shlex
import subprocess
import re
from typing import List, TypedDict
import cmd2
from cmd2 import Cmd, CommandSet
from cmd2 import Cmd2ArgumentParser, with_argparser


class BondingMemberT(TypedDict):
    name: str
    status: str
    active: bool


class BondingInfosT(TypedDict):
    members: List[BondingMemberT]
    mode: str


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


def list_sw_ports(name: str) -> List[str]:
    ports = run_command(f"ovs-vsctl list-ports {name}", check_output=True)
    return [p for p in ports.split("\n") if p != '']


def is_sw_port_exist(sw_name: str, port: int) -> bool:
    ports = list_sw_ports(sw_name)
    return f"{sw_name}.{port}" in ports


def parse_bonding_infos(sw_name: str, bond_name: str) -> BondingInfosT:
    try:
        infos = run_command(
            f"ovs-appctl bond/show {sw_name}.{bond_name}",
            check_output=True)
    except ConsoleError as err:
        raise ConsoleError(f"Unable to get infos on bonding interface {bond_name}: {err}")
    else:
        result: BondingInfosT = {
            "mode": "",
            "members": [],
        }

        for line in infos.split("\n"):
            if line.startswith("bond_mode: "):
                mode = line.replace("bond_mode: ", "")
                result['mode'] = mode
            elif line.startswith("member "):
                groups = re.match(r"^member\s+(\S+): (\S+)$", line)
                if groups is None:
                    continue
                ifname = groups[1].split(".")[1]
                result["members"].append({
                    "name": ifname,
                    "status": groups[2],
                    "active": False,
                })
        
        return result


def format_vlan_tags(v: str) -> str:
    if re.match(r"^[\d|,]+$", v) is None:
        raise argparse.ArgumentTypeError("")
    return v


@cmd2.with_category("vlan")
class OvsVlanCommandSet(CommandSet):

    def __init__(self, sw_name: str):
        super().__init__()
        self.sw_name = sw_name


    vlan_show_parser = cmd2.Cmd2ArgumentParser()

    @cmd2.as_subcommand_to('show', 'vlan', vlan_show_parser)
    def show_vlan(self, _: argparse.Namespace):
        """Show actual VLAN configuration"""
        ports = list_sw_ports(self.sw_name)
        for port in ports:
            if not port.startswith(self.sw_name+"."):
                continue
            self._cmd.poutput(f"Port {port.split('.')[1]}")

            try:
                # get vlan
                tag = run_command(f"ovs-vsctl get port {port} tag", check_output=True)
                if tag == "[]":
                    tag = "0"
                self._cmd.poutput(f"  VLAN Access: {tag}")
                # get trunks
                trunks = run_command(f"ovs-vsctl get port {port} trunks", check_output=True)
                if trunks != "[]":
                    self._cmd.poutput(f"  VLAN Trunk: {trunks.strip('[]')}")
            except ConsoleError as err:
                self._cmd.perror("Unable to get port info: {}".format(err))


    vlan_set_parser = cmd2.Cmd2ArgumentParser()
    vlan_set_parser.add_argument('port_alias', choices=["port"]) 
    vlan_set_parser.add_argument('port', type=str) 
    vlan_set_parser.add_argument('type', type=str, choices=["access", "trunk"]) 
    vlan_set_parser.add_argument('tag', type=format_vlan_tags) 

    @cmd2.as_subcommand_to('set', 'vlan', vlan_set_parser)
    def set_vlan(self, args: argparse.Namespace):
        """Configure VLAN on a port in access or trunk mode
           ex: set vlan port 0 access 10
           ex: set vlan port 0 trunk 20,30
        """
        if args.type == "access":
            try:
                run_command(f"ovs-vsctl set port {self.sw_name}.{args.port} tag={args.tag}")
            except ConsoleError as err:
                self._cmd.perror(f"Unable add port {args.port} to vlan {args.tag} in access mode: {err}")

        if args.type == "trunk":
            try:
                run_command(
                    f"ovs-vsctl set port {self.sw_name}.{args.port} trunks={args.tag}"
                )
            except ConsoleError as err:
                self._cmd.perror( f"Unable add port {args.port} to trunks {args.tag}: {err}")

    vlan_delete_parser = cmd2.Cmd2ArgumentParser()
    vlan_delete_parser.add_argument('port_alias', choices=["port"]) 
    vlan_delete_parser.add_argument('port', type=str) 
    vlan_delete_parser.add_argument('type', type=str, choices=["access", "trunk"]) 
    vlan_delete_parser.add_argument('tag', type=format_vlan_tags) 

    @cmd2.as_subcommand_to('delete', 'vlan', vlan_delete_parser)
    def delete_vlan(self, args: argparse.Namespace):
        """Remove a port to a VLAN (access/trunk mode)
           ex: delete vlan port 0 access 10
           ex: delete vlan port 0 trunk 20,30
        """
        if args.type == "access":
            try:
                run_command(f"ovs-vsctl remove port {self.sw_name}.{args.port} tag={args.tag}")
            except ConsoleError as err:
                self._cmd.perror(f"Unable remove port {args.port} to vlan {args.tag} in access mode: {err}")

        if args.type == "trunk":
            try:
                run_command(f"ovs-vsctl remove port {self.sw_name}.{args.port} trunks={args.tag}")
            except ConsoleError as err:
                self._cmd.perror(f"Unable remove port {args.port} to vlan(s) {args.tag} in trunk mode: {err}")


@cmd2.with_category("bonding")
class OvsBondingCommandSet(CommandSet):

    def __init__(self, sw_name: str):
        super().__init__()
        self.sw_name = sw_name

    bonding_show_parser = cmd2.Cmd2ArgumentParser()
    bonding_show_parser.add_argument('name', help='name of the bonding')

    @cmd2.as_subcommand_to('show', 'bonding', bonding_show_parser)
    def show_bonding(self, args: argparse.Namespace):
        """Show detail on a bond interface"""
        try:
            infos = parse_bonding_infos(self.sw_name, args.name)
        except ConsoleError as err:
            self._cmd.perror(f"Unable to get infos on bonding interface {args.name}: {err}")
        else:
            self._cmd.poutput(f"Mode: {infos['mode']}")
            self._cmd.poutput("Members:")
            for member in infos["members"]:
                self._cmd.poutput(f"\t{member['name']}: {member['status']}")

    bonding_add_argparser = Cmd2ArgumentParser()
    bonding_add_argparser.add_argument('name', help='name of the bonding')
    bonding_add_argparser.add_argument('port_alias', choices=["port"]) 
    bonding_add_argparser.add_argument('ifaces', nargs="+", help='interfaces to attach to the created bonding', type=int)

    @cmd2.as_subcommand_to('set', 'bonding', bonding_add_argparser)
    def add_bonding(self, args):
        """Create a bond interface (with LACP active) and attach ifaces
           ex: set bonding <my-bond> port 0 2
        """
        if len(args.ifaces) < 2:
            self._cmd.perror("A least 2 interfaces is required")
            return

        # check that ifaces belong
        for iface in args.ifaces:
            if not is_sw_port_exist(self.sw_name, iface):
                self._cmd.perror(f"Interface {iface} does not exist on this switch")
                return

        try:
            for iface in args.ifaces:
                run_command(f"ovs-vsctl del-port {self.sw_name} {self.sw_name}.{iface}")

            ifaces = " ".join([f"{self.sw_name}.{i}" for i in args.ifaces])
            run_command(f"ovs-vsctl add-bond {self.sw_name} {self.sw_name}.{args.name} {ifaces} lacp=active")
        except ConsoleError as err:
            self._cmd.perror(f"Unable to create bonding {args.name}: {err}")

    bonding_del_argparser = Cmd2ArgumentParser()
    bonding_del_argparser.add_argument('name', help='name of the bonding')

    @cmd2.as_subcommand_to('delete', 'bonding', bonding_del_argparser)
    def delete_bonding(self, args: argparse.Namespace):
        """Delete a bond interface
           ex: delete bonding <my-bond>
        """
        try:
            # first get bond members in order to readd to switch
            members = parse_bonding_infos(self.sw_name, args.name)["members"]

            run_command(f"ovs-vsctl del-port {self.sw_name} {self.sw_name}.{args.name}")
            for member in members:
                run_command(f"ovs-vsctl add-port {self.sw_name} {self.sw_name}.{member['name']}")
        except ConsoleError as err:
            self._cmd.perror(f"Unable to delete bonding {args.name}: {err}")


class OvsConsole(Cmd):
    intro = "Welcome to switch console. Type help or ? to list commands."

    def __init__(self, sw_name: str):
        self.prompt = f"[{sw_name}]>"
        super(OvsConsole, self).__init__(
            allow_cli_args=False,
            auto_load_commands=False)
        # disable some commands
        disable_commmands = [
            "edit", "py", "set", "run_pyscript", "run_script",
            "shortcuts", "shell", "macro", "alias"]
        for cmd_name in disable_commmands:
            if hasattr(Cmd, "do_"+cmd_name):
                delattr(Cmd, "do_"+cmd_name)

        self.sw_name = sw_name

        # load command set
        self.register_command_set(OvsVlanCommandSet(sw_name))
        self.register_command_set(OvsBondingCommandSet(sw_name))

    def emptyline(self):
        # do nothing when an empty line is entered
        pass

    def do_exit(self, _: str):
        """Alias for quit"""
        return True

    show_parser = cmd2.Cmd2ArgumentParser()
    show_subparsers = show_parser.add_subparsers(title='item', help='config part to show')

    @with_argparser(show_parser)
    def do_show(self, ns: argparse.Namespace):
        """show Vlan|Bonding configuration"""
        handler = ns.cmd2_handler.get()
        if handler is not None:
            # Call whatever subcommand function was selected
            handler(ns)
        else:
            # No subcommand was provided, so call help
            self.poutput('This command does nothing without sub-parsers registered')
            self.do_help('show')


    set_parser = cmd2.Cmd2ArgumentParser()
    set_subparsers = set_parser.add_subparsers(title='item', help='object part to configure')

    @with_argparser(set_parser)
    def do_set(self, ns: argparse.Namespace):
        """set Vlan|Bonding configuration"""
        handler = ns.cmd2_handler.get()
        if handler is not None:
            # Call whatever subcommand function was selected
            handler(ns)
        else:
            # No subcommand was provided, so call help
            self.poutput('This command does nothing without sub-parsers registered')
            self.do_help('set')

    delete_parser = cmd2.Cmd2ArgumentParser()
    delete_subparsers = delete_parser.add_subparsers(title='item', help='object part to configure')

    @with_argparser(delete_parser)
    def do_delete(self, ns: argparse.Namespace):
        """delete Vlan|Bonding configuration"""
        handler = ns.cmd2_handler.get()
        if handler is not None:
            # Call whatever subcommand function was selected
            handler(ns)
        else:
            # No subcommand was provided, so call help
            self.poutput('This command does nothing without sub-parsers registered')
            self.do_help('delete')


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
