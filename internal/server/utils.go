package server

import "fmt"

func ShortName(name string) string {
	if len(name) <= 2 {
		return name
	}

	return name[len(name)-3 : len(name)-1]
}

func GenIfName(prjId, hostName, peerName string, hostIfIdx, peerIfIdx int) string {
	return fmt.Sprintf(
		"%s%s%d.%s%d", prjId, ShortName(hostName), hostIfIdx,
		ShortName(peerName), peerIfIdx)
}
