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

func formatPercent(per float64) uint32 {
	perF := per / 100.0
	result := uint32(float64(uint32Max) * perF)

	return result
}

func formatTime(t int) uint32 {
	// TODO: understand why we need 15.625 factor
	return uint32(float64(t) * 1000 * 15.625)
}

func netemQdisc(devID netlink.Link, delay int, jitter int, loss float64) tc.Object {
	return tc.Object{
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
					Latency: formatTime(delay),
					Jitter:  formatTime(jitter),
					Limit:   1000,
					Loss:    formatPercent(loss),
				},
			},
		},
	}
}

func Netem(ifname string, namespace netns.NsHandle, delay int, jitter int, loss float64, change bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)

	// get interface ID
	devID, err := netlink.LinkByName(ifname)
	if err != nil {
		return fmt.Errorf("could not get interface ID for %s: %v", ifname, err)
	}

	// open a rtnetlink socket
	rtnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return fmt.Errorf("could not open rtnetlink socket: %v", err)
	}
	defer func() {
		if err := rtnl.Close(); err != nil {
			logger.Errorf("Could not close rtnetlink socket: %v", err)
		}
	}()

	qdisc := netemQdisc(devID, delay, jitter, loss)
	if !change {
		// tc qdisc add dev ifname root netem ...
		if err := rtnl.Qdisc().Add(&qdisc); err != nil {
			return fmt.Errorf("could not assign qdisc netem to %s: %v", ifname, err)
		}
	} else {
		// tc qdisc change dev ifname root netem ...
		if err := rtnl.Qdisc().Change(&qdisc); err != nil {
			return fmt.Errorf("could not assign qdisc netem to %s: %v", ifname, err)
		}
	}

	return nil
}

func CreateTbf(ifname string, namespace netns.NsHandle, delay, rate int, bufFactor float64, change bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netns.Set(namespace)

	// get interface ID
	devID, err := netlink.LinkByName(ifname)
	if err != nil {
		return fmt.Errorf("could not get interface ID for %s: %v", ifname, err)
	}

	// open a rtnetlink socket
	rtnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return fmt.Errorf("could not open rtnetlink socket: %v", err)
	}
	defer func() {
		if err := rtnl.Close(); err != nil {
			logger.Errorf("Could not close rtnetlink socket: %v", err)
		}
	}()

	linklayerEthernet := uint8(1)
	tbfBurst := uint32(rate * 4) // rate (in bps) / 250 HZ
	// limit (as rate) has to specified in bytes
	limit := uint32(bufFactor * float64(rate) * float64(delay) / 8.0) // rate * latency * BDPFactor

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

	if !change {
		// tc qdisc add dev ifname root netem ...
		if err := rtnl.Qdisc().Add(&qdisc); err != nil {
			return fmt.Errorf("could not assign qdisc tbf to %s: %v", ifname, err)
		}
	} else {
		// tc qdisc change dev ifname root netem ...
		if err := rtnl.Qdisc().Change(&qdisc); err != nil {
			return fmt.Errorf("could not change qdisc tbf to %s: %v", ifname, err)
		}
	}

	return nil
}
