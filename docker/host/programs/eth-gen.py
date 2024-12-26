#!/usr/bin/env python3
# https://docs.python.org/3.9/library/argparse.html

from scapy.all import *

import sys
import argparse
import socket
import netifaces

def parse_arguments():
    msg_default="Salut, je m'appelle "+socket.gethostname()

    parser = argparse.ArgumentParser(description='Argument Parser Template')
    parser.add_argument('-i', '--ifname', help='Nom de l\'interface de sortie', required=True)
    parser.add_argument('-d', '--dstaddr', help='Adresse MAC de destination', default='ff:ff:ff:ff:ff:ff')
    parser.add_argument('-m', '--msg', help='Message inclus dans la trame ethernet', type=str, default=msg_default)
    parser.add_argument('-t', '--type', help='Valeur du champs Type de la trame (defaut LOOPBACK 0x9000)', type=lambda x: int(x,0), default='0x9000')
    parser.add_argument('-v', '--verbose', help='increase output verbosity, flag', action='store_true')
    parser.add_argument('-tl', '--testlacp', help='Test de redondance LACP', action='store_true')
    parser.add_argument('-c', '--count', help='Nombre de trames émises (defaut=1)', type=int, default=1)
    parser.add_argument('-in', '--inter', help='Intervalle entre les trames', type=int, default=0)
    parser.add_argument('--ip', help='Trame ethernet de type 0x0800',action='store_true')
    parser.add_argument('--icmp', help='Trame etherbet de type 0x0800 et icmp',action='store_true')
    return parser.parse_args()

if __name__ == '__main__':

    args = parse_arguments()

    default_gateway='10.0.2.2'

    loop_default=0
    count_default=args.count
    inter_default=args.inter

    if args.ip:
        print(f'Trame ethernet de type 0x0800')
        mac = get_if_hwaddr (args.ifname)
        print(f'adresse mac source : {mac}')
        pkt=Ether(src=mac, dst=args.dstaddr)/ IP()
        sendp(pkt,iface=args.ifname,verbose=args.verbose, loop=loop_default, count=count_default, inter=inter_default)
        sys.exit(0)
    if args.icmp:
        print(f'Trame ethernet de type 0x0800 et icmp')
        mac = get_if_hwaddr (args.ifname)
        print(f'adresse mac source : {mac}')
        pkt=Ether(src=mac)/ IP() /ICMP()
        sendp(pkt,iface=args.ifname,verbose=args.verbose, loop=loop_default, count=count_default, inter=inter_default)
        sys.exit(0)
    if args.ifname:
        print(f'Interface de sortie : {args.ifname}')
    if args.dstaddr:
        print(f'Adresse de destination : {args.dstaddr}')
    if args.msg:
        print(f'message : {args.msg}')
    if args.verbose:
        print('verbosity flag enabled')
    if args.testlacp:
        print('Test redondance LACP')
        loop_default=1
        count_default=1000
        inter_default=1

    print("")
    print(f'Intervalle envoi : {inter_default}')
    typeframe = hex(args.type)
    print(f'type : {typeframe}')

    myframe=Ether(dst=args.dstaddr,type=args.type)/args.msg
    sendp(myframe,iface=args.ifname,verbose=args.verbose, loop=loop_default, count=count_default, inter=inter_default)
