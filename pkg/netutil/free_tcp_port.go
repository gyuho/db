package netutil

import (
	"fmt"
	"net"
	"sort"
)

func getFreeTCPPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	pd := l.Addr().(*net.TCPAddr).Port
	l.Close()

	return pd, nil
}

// GetFreeTCPPorts returns available ports.
func GetFreeTCPPorts(num int) ([]int, error) {
	try := 0
	rm := make(map[int]struct{})
	for len(rm) != num {
		if try > 500 {
			return nil, fmt.Errorf("too many tries to find free ports")
		}

		p, err := getFreeTCPPort()
		if err != nil {
			return nil, err
		}
		rm[p] = struct{}{}

		try++
	}

	var ns []int
	for p := range rm {
		ns = append(ns, p)
	}
	sort.Ints(ns)

	return ns, nil
}
