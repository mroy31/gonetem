package link

import (
	"os"
	"runtime"
	"testing"

	"github.com/mroy31/gonetem/internal/utils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func skipUnlessRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Test requires root privileges.")
	}
}

func setUpNetlinkTest(t *testing.T) func() {
	skipUnlessRoot(t)

	// new temporary namespace so we don't pollute the host
	// lock thread since the namespace is thread local
	runtime.LockOSThread()
	var err error
	ns, err := netns.New()
	if err != nil {
		t.Fatal("Failed to create newns", ns)
	}

	return func() {
		ns.Close()
		runtime.UnlockOSThread()
	}
}

func TestLink_CreateVeth(t *testing.T) {
	teardown := setUpNetlinkTest(t)
	defer teardown()

	ns, err := netns.Get()
	if err != nil {
		t.Fatalf("Unable to get current netns")
	}

	veth, err := CreateVethLink(utils.RandString(6), ns, utils.RandString(6), ns)
	if err != nil {
		t.Fatalf("Unable to create veth: %v", err)
	}
	defer netlink.LinkDel(veth)

	// check existence
	_, err = netlink.LinkByName(veth.Name)
	if err != nil {
		t.Fatalf("Unable to find created veth: %v", err)
	}
	_, err = netlink.LinkByName(veth.PeerName)
	if err != nil {
		t.Fatalf("Unable to find created veth peer: %v", err)
	}
}

func TestLink_CreateVrf(t *testing.T) {
	teardown := setUpNetlinkTest(t)
	defer teardown()

	ns, err := netns.Get()
	if err != nil {
		t.Fatalf("Unable to get current netns")
	}

	vrf, err := CreateVrf(utils.RandString(6), ns, 10)
	if err != nil {
		t.Fatalf("Unable to create VRF: %v", err)
	}
	defer netlink.LinkDel(vrf)

	// check existence
	_, err = netlink.LinkByName(vrf.Name)
	if err != nil {
		t.Fatalf("Unable to find created vrf: %v", err)
	}
}

func TestLink_InterfaceState(t *testing.T) {
	teardown := setUpNetlinkTest(t)
	defer teardown()

	ns, err := netns.Get()
	if err != nil {
		t.Fatalf("Unable to get current netns")
	}

	veth, err := CreateVethLink(utils.RandString(6), ns, utils.RandString(6), ns)
	if err != nil {
		t.Fatalf("Unable to create veth: %v", err)
	}
	defer netlink.LinkDel(veth)

	// check existence
	_, err = netlink.LinkByName(veth.Name)
	if err != nil {
		t.Fatalf("Unable to find created veth: %v", err)
	}

	// set interface up
	if err := SetInterfaceState(veth.Name, ns, IFSTATE_UP); err != nil {
		t.Fatalf("%v", err)
	}
}
