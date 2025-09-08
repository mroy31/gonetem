package link

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type IfState int

const (
	IFSTATE_UP IfState = iota
	IFSTATE_DOWN
)

var (
	mutex = &sync.Mutex{}
)

func GetRootNetns() netns.NsHandle {
	ns, _ := netns.GetFromPid(os.Getpid())

	return ns
}

func IsLinkExist(name string, namespace netns.NsHandle) bool {
	mutex.Lock()
	defer mutex.Unlock()

	netns.Set(namespace)
	_, err := netlink.LinkByName(name)
	return err == nil
}

func CreateVethLink(name string, namespace netns.NsHandle, peerName string, peerNamespace netns.NsHandle) (*netlink.Veth, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:      name,
			MTU:       1500,
			TxQLen:    1000,
			Namespace: netlink.NsFd(namespace),
		},
		PeerName:      peerName,
		PeerNamespace: netlink.NsFd(peerNamespace),
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return nil, fmt.Errorf("error when creating Veth: %v", err)
	}

	return veth, nil
}

func CreateNetns(name string) (netns.NsHandle, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	return netns.NewNamed(name)
}

func DeleteNetns(name string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	return netns.DeleteNamed(name)
}

func CreateBridge(name string, namespace netns.NsHandle) (*netlink.Bridge, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)

	la := netlink.NewLinkAttrs()
	la.Name = name
	la.Namespace = netlink.NsFd(namespace)
	br := &netlink.Bridge{LinkAttrs: la}

	err := netlink.LinkAdd(br)
	if err != nil {
		return br, fmt.Errorf("error when creating bridge %s: %v", name, err)
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return br, fmt.Errorf("error when set %s up: %v", name, err)
	}

	return br, nil
}

func CreateMacVlan(name string, parent string, peerMAC net.HardwareAddr, mode netlink.MacvlanMode, namespace netns.NsHandle) (*netlink.Macvlan, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)
	parentLink, err := netlink.LinkByName(parent)
	if err != nil {
		return &netlink.Macvlan{}, fmt.Errorf("unable to find macvlan parent %s: %v", parent, err)
	}

	la := netlink.NewLinkAttrs()
	la.Name = name
	la.Namespace = netlink.NsFd(namespace)
	la.ParentIndex = parentLink.Attrs().Index
	la.HardwareAddr = peerMAC
	macvlan := &netlink.Macvlan{
		LinkAttrs: la,
		Mode:      mode,
	}

	if err := netlink.LinkAdd(macvlan); err != nil {
		return macvlan, fmt.Errorf("error when creating MACVLAN %s: %v", name, err)
	}
	return macvlan, nil
}

func CreateVRRPMacVlan(name string, parent string, group int, namespace netns.NsHandle) (*netlink.Macvlan, error) {
	peerMAC, err := net.ParseMAC(fmt.Sprintf("00:00:5E:00:01:%02X", group))
	if err != nil {
		return nil, err
	}

	return CreateMacVlan(name, parent, peerMAC, netlink.MACVLAN_MODE_BRIDGE, namespace)
}

func CreateVrf(name string, namespace netns.NsHandle, table int) (*netlink.Vrf, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)

	la := netlink.NewLinkAttrs()
	la.Name = name
	la.Namespace = netlink.NsFd(namespace)
	vrf := &netlink.Vrf{LinkAttrs: la, Table: uint32(table)}

	err := netlink.LinkAdd(vrf)
	if err != nil {
		return vrf, fmt.Errorf("error when creating VRF %s: %v", name, err)
	}

	if err := netlink.LinkSetUp(vrf); err != nil {
		return vrf, fmt.Errorf("error when set %s up: %v", name, err)
	}

	return vrf, nil
}

func AttachToBridge(br *netlink.Bridge, ifName string, namespace netns.NsHandle) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)
	ifObj, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("unable to get %s: %v", ifName, err)
	}

	return netlink.LinkSetMaster(ifObj, br)
}

func DeleteLink(name string, namespace netns.NsHandle) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)
	br, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("unable to get link %s: %v", name, err)
	}

	return netlink.LinkDel(br)
}

func RenameLink(name string, target string, namespace netns.NsHandle) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(namespace); err != nil {
		return fmt.Errorf("renameLink - Error when switching netns: %v", err)
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("renameLink - Unable get link %s: %v", name, err)
	}

	if err := netlink.LinkSetName(link, target); err != nil {
		return fmt.Errorf("error when renaming link %s->%s: %v", name, target, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("error when set %s up: %v", name, err)
	}

	return nil
}

func SetInterfaceState(name string, namespace netns.NsHandle, state IfState) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(namespace); err != nil {
		return fmt.Errorf("error when switching netns: %v", err)
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("unable get link %s: %v", name, err)
	}

	switch state {
	case IFSTATE_UP:
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("error when set %s up: %v", name, err)
		}
	case IFSTATE_DOWN:
		if err := netlink.LinkSetDown(link); err != nil {
			return fmt.Errorf("error when set %s down: %v", name, err)
		}
	}

	return nil
}

func SetLinkNetns(link netlink.Link, targetNs netns.NsHandle) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	return netlink.LinkSetNsFd(link, int(targetNs))
}

func MoveInterfacesNetns(ifNames map[string]IfState, current netns.NsHandle, target netns.NsHandle) error {
	if len(ifNames) == 0 {
		return nil
	}

	// As we need to stay in the right namespace
	// Use mutex to avoid netns change
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(current); err != nil {
		return fmt.Errorf("error when switching netns: %v", err)
	}

	for ifName := range ifNames {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("unable get link %s: %v", ifName, err)
		}

		if err := netlink.LinkSetNsFd(link, int(target)); err != nil {
			return fmt.Errorf("error when update netns for %s: %v", ifName, err)
		}
	}

	return nil
}

func IpAddressAdd(ifName string, namespace netns.NsHandle, IPAddress string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(namespace); err != nil {
		return fmt.Errorf("error when switching netns: %v", err)
	}

	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("unable get link %s: %v", ifName, err)
	}

	addr, err := netlink.ParseAddr(IPAddress)
	if err != nil {
		return fmt.Errorf("unable to parse IP address %s: %v", IPAddress, err)
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("unable to add IP address %s to link %s: %v", IPAddress, ifName, err)
	}

	return nil
}
