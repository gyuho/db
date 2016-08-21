package netutil

import (
	"fmt"
	"testing"
)

func Test_getFreeTCPPort(t *testing.T) {
	pt, err := getFreeTCPPort()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("getFreeTCPPort: %d\n", pt)
}

func Test_GetFreeTCPPorts(t *testing.T) {
	ps, err := GetFreeTCPPorts(3)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("GetFreeTCPPorts: %+v\n", ps)
}
