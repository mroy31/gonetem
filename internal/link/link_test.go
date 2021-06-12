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

func setUpNetlinkTest(t *testing.T) (netns.NsHandle, func()) {
	skipUnlessRoot(t)

	// new temporary namespace so we don't pollute the host
	// lock thread since the namespace is thread local
	runtime.LockOSThread()
	var err error
	ns, err := netns.New()
	if err != nil {
		t.Fatal("Failed to create newns", ns)
	}

	return ns, func() {
		ns.Close()
		runtime.UnlockOSThread()
	}
}

func checkLinkExistence(t *testing.T, lkNames ...string) {
	for _, lkName := range lkNames {
		_, err := netlink.LinkByName(lkName)
		if err != nil {
			t.Fatalf("Unable to find created vrf: %v", lkName)
		}
	}
}

func TestLink_CreateVeth(t *testing.T) {
	ns, teardown := setUpNetlinkTest(t)
	defer teardown()

	veth, err := CreateVethLink(utils.RandString(6), ns, utils.RandString(6), ns)
	if err != nil {
		t.Fatalf("Unable to create veth: %v", err)
	}
	defer netlink.LinkDel(veth)

	checkLinkExistence(t, veth.Name, veth.PeerName)
}

func TestLink_CreateBridge(t *testing.T) {
	ns, teardown := setUpNetlinkTest(t)
	defer teardown()

	br, err := CreateBridge(utils.RandString(5), ns)
	if err != nil {
		t.Fatalf("Unable to create veth: %v", err)
	}
	defer netlink.LinkDel(br)

	checkLinkExistence(t, br.Name)
}

func TestLink_CreateMacvlan(t *testing.T) {
	ns, teardown := setUpNetlinkTest(t)
	defer teardown()

	parent := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "foo"}}
	if err := netlink.LinkAdd(parent); err != nil {
		t.Fatal(err)
	}
	defer netlink.LinkDel(parent)

	macvlan, err := CreateMacVlan(utils.RandString(5), "foo", 1, ns)
	if err != nil {
		t.Fatalf("Unable to create macvlan: %v", err)
	}
	defer netlink.LinkDel(macvlan)

	checkLinkExistence(t, macvlan.Name)
}

func TestLink_CreateVrf(t *testing.T) {
	ns, teardown := setUpNetlinkTest(t)
	defer teardown()

	vrf, err := CreateVrf(utils.RandString(6), ns, 10)
	if err != nil {
		t.Fatalf("Unable to create VRF: %v", err)
	}
	defer netlink.LinkDel(vrf)

	checkLinkExistence(t, vrf.Name)
}

func TestLink_InterfaceState(t *testing.T) {
	ns, teardown := setUpNetlinkTest(t)
	defer teardown()

	veth, err := CreateVethLink(utils.RandString(6), ns, utils.RandString(6), ns)
	if err != nil {
		t.Fatalf("Unable to create veth: %v", err)
	}
	defer netlink.LinkDel(veth)

	checkLinkExistence(t, veth.Name, veth.PeerName)

	// set interface up
	if err := SetInterfaceState(veth.Name, ns, IFSTATE_UP); err != nil {
		t.Fatalf("%v", err)
	}
}
