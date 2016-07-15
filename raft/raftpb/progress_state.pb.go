// Code generated by protoc-gen-gogo.
// source: raft/raftpb/progress_state.proto
// DO NOT EDIT!

package raftpb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// PROGRESS_STATE represents the state of follower progress, in leader's viewpoint.
// Leader maintains progresses of all followers, and replicates entries to the followers
// based on their progresses. Follower.Progress has 'MatchIndex' and 'NextIndex'.
// 'MatchIndex' is the highest known matched index of follower entries.
// If unknown, it is set to 0. Then leader replicates entries from 'NextIndex'
// to its last one.
type PROGRESS_STATE int32

const (
	// When a follower is in PROBE state, leader sends at most one message
	// for each heartbeat interval. This is to probe the actual progress of
	// the follower.
	PROGRESS_STATE_PROBE PROGRESS_STATE = 0
	// When a follower is in REPLICATE state, leader replicates messages
	// by optimistically increasing 'NextIndex' to latest. This is an
	// optimized state for fast log replication. If it fails, leader sets
	// it back to PROBE.
	// The maximum number of entries per message and the maximum number of
	// inflight messages are configurable.
	PROGRESS_STATE_REPLICATE PROGRESS_STATE = 1
	// Leader changes follower's state from PROBE to SNAPSHOT
	// when the follower has fallen too far behind the leader,
	// and sends snapshot to the follower. Then leader must wait
	// until the success. After snapshot is applied, leader sets
	// it back to PROBE.
	PROGRESS_STATE_SNAPSHOT PROGRESS_STATE = 2
)

var PROGRESS_STATE_name = map[int32]string{
	0: "PROBE",
	1: "REPLICATE",
	2: "SNAPSHOT",
}
var PROGRESS_STATE_value = map[string]int32{
	"PROBE":     0,
	"REPLICATE": 1,
	"SNAPSHOT":  2,
}

func (x PROGRESS_STATE) String() string {
	return proto.EnumName(PROGRESS_STATE_name, int32(x))
}
func (PROGRESS_STATE) EnumDescriptor() ([]byte, []int) { return fileDescriptorProgressState, []int{0} }

func init() {
	proto.RegisterEnum("raftpb.PROGRESS_STATE", PROGRESS_STATE_name, PROGRESS_STATE_value)
}

var fileDescriptorProgressState = []byte{
	// 176 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xe2, 0x52, 0x28, 0x4a, 0x4c, 0x2b,
	0xd1, 0x07, 0x11, 0x05, 0x49, 0xfa, 0x05, 0x45, 0xf9, 0xe9, 0x45, 0xa9, 0xc5, 0xc5, 0xf1, 0xc5,
	0x25, 0x89, 0x25, 0xa9, 0x7a, 0x05, 0x45, 0xf9, 0x25, 0xf9, 0x42, 0x6c, 0x10, 0x49, 0x29, 0xdd,
	0xf4, 0xcc, 0x92, 0x8c, 0xd2, 0x24, 0xbd, 0xe4, 0xfc, 0x5c, 0xfd, 0xf4, 0xfc, 0xf4, 0x7c, 0x7d,
	0xb0, 0x74, 0x52, 0x69, 0x1a, 0x98, 0x07, 0xe6, 0x80, 0x59, 0x10, 0x6d, 0x5a, 0x16, 0x5c, 0x7c,
	0x01, 0x41, 0xfe, 0xee, 0x41, 0xae, 0xc1, 0xc1, 0xf1, 0xc1, 0x21, 0x8e, 0x21, 0xae, 0x42, 0x9c,
	0x5c, 0xac, 0x01, 0x41, 0xfe, 0x4e, 0xae, 0x02, 0x0c, 0x42, 0xbc, 0x5c, 0x9c, 0x41, 0xae, 0x01,
	0x3e, 0x9e, 0xce, 0x8e, 0x21, 0xae, 0x02, 0x8c, 0x42, 0x3c, 0x5c, 0x1c, 0xc1, 0x7e, 0x8e, 0x01,
	0xc1, 0x1e, 0xfe, 0x21, 0x02, 0x4c, 0x4e, 0x22, 0x27, 0x1e, 0xca, 0x31, 0x9c, 0x78, 0x24, 0xc7,
	0x78, 0xe1, 0x91, 0x1c, 0xe3, 0x83, 0x47, 0x72, 0x8c, 0x33, 0x1e, 0xcb, 0x31, 0x24, 0xb1, 0x81,
	0x8d, 0x35, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0x10, 0x0b, 0x20, 0x9b, 0xb1, 0x00, 0x00, 0x00,
}
