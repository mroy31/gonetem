package link

import (
	"fmt"
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
		return nil, fmt.Errorf("Error when creating Veth: %v", err)
	}

	return veth, nil
}

func CreateBridge(name string, namespace netns.NsHandle) (*netlink.Bridge, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	la := netlink.NewLinkAttrs()
	la.Name = name
	la.Namespace = netlink.NsFd(namespace)
	br := &netlink.Bridge{LinkAttrs: la}

	err := netlink.LinkAdd(br)
	if err != nil {
		return br, fmt.Errorf("Error when creating bridge %s: %v", name, err)
	}

	netns.Set(namespace)
	if err := netlink.LinkSetUp(br); err != nil {
		return br, fmt.Errorf("Error when set %s up: %v", name, err)
	}

	return br, nil
}

func CreateVrf(name string, namespace netns.NsHandle, table int) (*netlink.Vrf, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	la := netlink.NewLinkAttrs()
	la.Name = name
	la.Namespace = netlink.NsFd(namespace)
	vrf := &netlink.Vrf{LinkAttrs: la, Table: uint32(table)}

	err := netlink.LinkAdd(vrf)
	if err != nil {
		return vrf, fmt.Errorf("Error when creating VRF %s: %v", name, err)
	}

	netns.Set(namespace)
	if err := netlink.LinkSetUp(vrf); err != nil {
		return vrf, fmt.Errorf("Error when set %s up: %v", name, err)
	}

	return vrf, nil
}

func AttachToBridge(br *netlink.Bridge, ifName string, namespace netns.NsHandle) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)
	ifObj, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("Unable to get %s: %v", ifName, err)
	}

	return netlink.LinkSetMaster(ifObj, br)
}

func DeleteLink(name string, namespace netns.NsHandle) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)
	br, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("Unable to get link %s: %v", name, err)
	}

	return netlink.LinkDel(br)
}

func RenameLink(name string, target string, namespace netns.NsHandle) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(namespace); err != nil {
		return fmt.Errorf("RenameLink - Error when switching netns: %v", err)
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("RenameLink - Unable get link %s: %v", name, err)
	}

	if err := netlink.LinkSetName(link, target); err != nil {
		return fmt.Errorf("Error when renaming link %s->%s: %v", name, target, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("Error when set %s up: %v", name, err)
	}

	return nil
}

func SetInterfaceState(name string, namespace netns.NsHandle, state IfState) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(namespace); err != nil {
		return fmt.Errorf("Error when switching netns: %v", err)
	}

	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("Unable get link %s: %v", name, err)
	}

	switch state {
	case IFSTATE_UP:
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("Error when set %s up: %v", name, err)
		}
	case IFSTATE_DOWN:
		if err := netlink.LinkSetDown(link); err != nil {
			return fmt.Errorf("Error when set %s down: %v", name, err)
		}
	}

	return nil
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
		return fmt.Errorf("Error when switching netns: %v", err)
	}

	for ifName, _ := range ifNames {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("Unable get link %s: %v", ifName, err)
		}

		if err := netlink.LinkSetNsFd(link, int(target)); err != nil {
			return fmt.Errorf("Error when update netns for %s: %v", ifName, err)
		}
	}

	return nil
}
