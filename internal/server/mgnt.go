package server

import (
	"fmt"

	"github.com/mroy31/gonetem/internal/link"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type MgntNetwork struct {
	NetId      string
	IPAddress  string
	Instance   *netlink.Bridge
	NetNs      netns.NsHandle
	Interfaces []string
	Logger     *logrus.Entry
}

func (mNet *MgntNetwork) Create() error {
	var err error

	mNet.Instance, err = link.CreateBridge(mNet.NetId, mNet.NetNs)
	if err != nil {
		return fmt.Errorf("unable to create management bridge %s: %v", mNet.NetId, err)
	}

	if err := link.IpAddressAdd(mNet.NetId, mNet.NetNs, mNet.IPAddress); err != nil {
		return fmt.Errorf("unable to assign IP address %s to mgnt network: %v", mNet.IPAddress, err)
	}

	return nil
}

func (mNet *MgntNetwork) AttachInterface(ifName string) error {
	if mNet.Instance == nil {
		return fmt.Errorf("mgnt network is not created")
	}

	if err := link.AttachToBridge(mNet.Instance, ifName, mNet.NetNs); err != nil {
		return fmt.Errorf("unable to attach %s to mgnt network: %v", ifName, err)
	}
	mNet.Interfaces = append(mNet.Interfaces, ifName)

	return nil
}

func (mNet *MgntNetwork) Close() error {
	for _, ifName := range mNet.Interfaces {
		if err := link.DeleteLink(ifName, mNet.NetNs); err != nil {
			mNet.Logger.Warnf("Error when deleting link %s: %v", ifName, err)
		}
	}
	mNet.Interfaces = make([]string, 0)

	if mNet.Instance != nil {
		return link.DeleteLink(mNet.NetId, mNet.NetNs)
	}

	return nil
}
