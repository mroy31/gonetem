package link

import (
	"fmt"
	"runtime"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const (
	uint32Max uint32 = 4294967295
)

var (
	logger = logrus.WithField("module", "tc")
)

func formatPercent(per int) uint32 {
	perF := float64(per) / 100.0
	result := uint32(float64(uint32Max) * perF)

	return result
}

func CreateNetem(ifname string, namespace netns.NsHandle, delay int, jitter int, loss int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)

	// get interface ID
	devID, err := netlink.LinkByName(ifname)
	if err != nil {
		return fmt.Errorf("Could not get interface ID for %s: %v\n", ifname, err)
	}

	// open a rtnetlink socket
	rtnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return fmt.Errorf("Could not open rtnetlink socket: %v", err)
	}
	defer func() {
		if err := rtnl.Close(); err != nil {
			logger.Errorf("Could not close rtnetlink socket: %v", err)
		}
	}()

	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(devID.Attrs().Index),
			Handle:  core.BuildHandle(0x1, 0x0),
			Parent:  tc.HandleRoot,
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind: "netem",
			Netem: &tc.Netem{
				Qopt: tc.NetemQopt{
					Latency: uint32(delay * 10000),  // ms
					Jitter:  uint32(jitter * 10000), // ms
					Limit:   1000,
					Loss:    formatPercent(loss),
				},
			},
		},
	}

	// tc qdisc add dev ifname root netem ...
	if err := rtnl.Qdisc().Add(&qdisc); err != nil {
		return fmt.Errorf("Could not assign qdisc netem to %s: %v\n", ifname, err)
	}

	return nil
}

func CreateTbf(ifname string, namespace netns.NsHandle, delay, rate int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)

	// get interface ID
	devID, err := netlink.LinkByName(ifname)
	if err != nil {
		return fmt.Errorf("Could not get interface ID for %s: %v\n", ifname, err)
	}

	// open a rtnetlink socket
	rtnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return fmt.Errorf("Could not open rtnetlink socket: %v", err)
	}
	defer func() {
		if err := rtnl.Close(); err != nil {
			logger.Errorf("Could not close rtnetlink socket: %v", err)
		}
	}()

	linklayerEthernet := uint8(1)
	tbfBurst := uint32(rate * 4)  // rate (in bps) / 250 HZ
	limit := uint32(rate * delay) // rate * latency

	qdisc := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(devID.Attrs().Index),
			Handle:  core.BuildHandle(0x10, 0x0),
			Parent:  core.BuildHandle(0x1, 0x1),
			Info:    0,
		},
		Attribute: tc.Attribute{
			Kind: "tbf",
			Tbf: &tc.Tbf{
				Parms: &tc.TbfQopt{
					Mtu:   1514,
					Limit: limit,
					Rate: tc.RateSpec{
						Rate:      uint32(rate * 125),
						Linklayer: linklayerEthernet,
						CellLog:   0x3,
					},
				},
				Burst: &tbfBurst,
			},
		},
	}

	// tc qdisc add dev ifname root netem ...
	if err := rtnl.Qdisc().Add(&qdisc); err != nil {
		return fmt.Errorf("Could not assign qdisc tbf to %s: %v\n", ifname, err)
	}

	return nil
}
