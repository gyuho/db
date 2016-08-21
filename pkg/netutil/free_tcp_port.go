package netutil

import (
	"fmt"
	"net"
	"sort"
	"strconv"
)

func getFreeTCPPort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}

	pd := l.Addr().(*net.TCPAddr).Port
	l.Close()

	return ":" + strconv.Itoa(pd), nil
}

// GetFreeTCPPorts returns available ports.
func GetFreeTCPPorts(num int) ([]string, error) {
	try := 0
	rm := make(map[string]struct{})
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

	var rs []string
	for p := range rm {
		rs = append(rs, p)
	}
	sort.Strings(rs)

	return rs, nil
}
