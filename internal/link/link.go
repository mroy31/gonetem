package link

import (
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func CreateVethLink(name, peerName string) (*netlink.Veth, error) {
	la := netlink.NewLinkAttrs()
	la.Name = name

	veth := &netlink.Veth{LinkAttrs: la, PeerName: peerName}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	return veth, nil
}

func AttachToPid(pid int, ifName, targetName string) error {
	origin, _ := netns.Get()
	defer origin.Close()

	ns, err := netns.GetFromPid(pid)
	if err != nil {
		return err
	}
	defer ns.Close()

	// add link to namespace
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return err
	}
	if err := netlink.LinkSetNsFd(link, int(ns)); err != nil {
		return err
	}

	netns.Set(ns)
	link, err = netlink.LinkByName(ifName)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetName(link, targetName); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}

	netns.Set(origin)
	return nil
}
