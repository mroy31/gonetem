package link

import (
	"fmt"
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

func SetInterfaceState(name string, namespace netns.NsHandle, state IfState) error {
	// As we need to stay in the right namespace
	// Use mutex to avoid netns change
	mutex.Lock()
	defer mutex.Unlock()

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

func MoveInterfacesNetns(ifNames []string, current netns.NsHandle, target netns.NsHandle) error {
	if len(ifNames) == 0 {
		return nil
	}

	// As we need to stay in the right namespace
	// Use mutex to avoid netns change
	mutex.Lock()
	defer mutex.Unlock()

	if err := netns.Set(current); err != nil {
		return fmt.Errorf("Error when switching netns: %v", err)
	}

	for _, ifName := range ifNames {
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