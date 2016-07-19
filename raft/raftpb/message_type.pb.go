// Code generated by protoc-gen-gogo.
// source: raft/raftpb/message_type.proto
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

// (etcd raft.raftpb.MessageType)
type MESSAGE_TYPE int32

const (
	MESSAGE_TYPE_INTERNAL_TRIGGER_CAMPAIGN                       MESSAGE_TYPE = 0
	MESSAGE_TYPE_INTERNAL_TRIGGER_LEADER_HEARTBEAT               MESSAGE_TYPE = 1
	MESSAGE_TYPE_INTERNAL_TRIGGER_CHECK_QUORUM                   MESSAGE_TYPE = 2
	MESSAGE_TYPE_INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER      MESSAGE_TYPE = 3
	MESSAGE_TYPE_LEADER_HEARTBEAT                                MESSAGE_TYPE = 4
	MESSAGE_TYPE_RESPONSE_TO_LEADER_HEARTBEAT                    MESSAGE_TYPE = 5
	MESSAGE_TYPE_CANDIDATE_REQUEST_VOTE                          MESSAGE_TYPE = 6
	MESSAGE_TYPE_RESPONSE_TO_CANDIDATE_REQUEST_VOTE              MESSAGE_TYPE = 7
	MESSAGE_TYPE_PROPOSAL_TO_LEADER                              MESSAGE_TYPE = 8
	MESSAGE_TYPE_LEADER_APPEND                                   MESSAGE_TYPE = 9
	MESSAGE_TYPE_RESPONSE_TO_LEADER_APPEND                       MESSAGE_TYPE = 10
	MESSAGE_TYPE_LEADER_SNAPSHOT                                 MESSAGE_TYPE = 11
	MESSAGE_TYPE_INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT            MESSAGE_TYPE = 12
	MESSAGE_TYPE_INTERNAL_TRANSFER_LEADER                        MESSAGE_TYPE = 13
	MESSAGE_TYPE_FORCE_ELECTION_TIMEOUT                          MESSAGE_TYPE = 14
	MESSAGE_TYPE_READ_LEADER_CURRENT_COMMITTED_INDEX             MESSAGE_TYPE = 15
	MESSAGE_TYPE_RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX MESSAGE_TYPE = 16
)

var MESSAGE_TYPE_name = map[int32]string{
	0:  "INTERNAL_TRIGGER_CAMPAIGN",
	1:  "INTERNAL_TRIGGER_LEADER_HEARTBEAT",
	2:  "INTERNAL_TRIGGER_CHECK_QUORUM",
	3:  "INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER",
	4:  "LEADER_HEARTBEAT",
	5:  "RESPONSE_TO_LEADER_HEARTBEAT",
	6:  "CANDIDATE_REQUEST_VOTE",
	7:  "RESPONSE_TO_CANDIDATE_REQUEST_VOTE",
	8:  "PROPOSAL_TO_LEADER",
	9:  "LEADER_APPEND",
	10: "RESPONSE_TO_LEADER_APPEND",
	11: "LEADER_SNAPSHOT",
	12: "INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT",
	13: "INTERNAL_TRANSFER_LEADER",
	14: "FORCE_ELECTION_TIMEOUT",
	15: "READ_LEADER_CURRENT_COMMITTED_INDEX",
	16: "RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX",
}
var MESSAGE_TYPE_value = map[string]int32{
	"INTERNAL_TRIGGER_CAMPAIGN":                       0,
	"INTERNAL_TRIGGER_LEADER_HEARTBEAT":               1,
	"INTERNAL_TRIGGER_CHECK_QUORUM":                   2,
	"INTERNAL_LEADER_CANNOT_CONNECT_TO_FOLLOWER":      3,
	"LEADER_HEARTBEAT":                                4,
	"RESPONSE_TO_LEADER_HEARTBEAT":                    5,
	"CANDIDATE_REQUEST_VOTE":                          6,
	"RESPONSE_TO_CANDIDATE_REQUEST_VOTE":              7,
	"PROPOSAL_TO_LEADER":                              8,
	"LEADER_APPEND":                                   9,
	"RESPONSE_TO_LEADER_APPEND":                       10,
	"LEADER_SNAPSHOT":                                 11,
	"INTERNAL_RESPONSE_TO_LEADER_SNAPSHOT":            12,
	"INTERNAL_TRANSFER_LEADER":                        13,
	"FORCE_ELECTION_TIMEOUT":                          14,
	"READ_LEADER_CURRENT_COMMITTED_INDEX":             15,
	"RESPONSE_TO_READ_LEADER_CURRENT_COMMITTED_INDEX": 16,
}

func (x MESSAGE_TYPE) String() string {
	return proto.EnumName(MESSAGE_TYPE_name, int32(x))
}
func (MESSAGE_TYPE) EnumDescriptor() ([]byte, []int) { return fileDescriptorMessageType, []int{0} }

func init() {
	proto.RegisterEnum("raftpb.MESSAGE_TYPE", MESSAGE_TYPE_name, MESSAGE_TYPE_value)
}

var fileDescriptorMessageType = []byte{
	// 424 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x8c, 0x52, 0xcd, 0x6e, 0x13, 0x31,
	0x10, 0x4e, 0xa0, 0x04, 0x18, 0x5a, 0x6a, 0x86, 0xaa, 0x82, 0xaa, 0x5d, 0x51, 0xfe, 0x55, 0x89,
	0xec, 0xa1, 0x4f, 0xe0, 0xee, 0x4e, 0x12, 0x8b, 0x5d, 0xdb, 0xb5, 0x67, 0xf9, 0x39, 0x59, 0x0d,
	0x4a, 0x03, 0x87, 0x6a, 0xa3, 0x36, 0x3d, 0xf0, 0x26, 0x3c, 0x10, 0x87, 0x1e, 0x79, 0x04, 0x08,
	0x2f, 0x82, 0x76, 0x1b, 0x56, 0x91, 0x12, 0x24, 0x2e, 0x96, 0xc7, 0xdf, 0xf7, 0xcd, 0x37, 0xf3,
	0xc9, 0x10, 0x9d, 0x9f, 0x9c, 0x4e, 0xe3, 0xea, 0x98, 0x0c, 0xe3, 0xb3, 0xd1, 0xc5, 0xc5, 0xc9,
	0x78, 0x14, 0xa6, 0x5f, 0x27, 0xa3, 0xee, 0xe4, 0xbc, 0x9c, 0x96, 0xd8, 0xb9, 0x86, 0x76, 0xde,
	0x8c, 0xbf, 0x4c, 0x3f, 0x5f, 0x0e, 0xbb, 0x9f, 0xca, 0xb3, 0x78, 0x5c, 0x8e, 0xcb, 0xb8, 0x86,
	0x87, 0x97, 0xa7, 0x75, 0x55, 0x17, 0xf5, 0xed, 0x5a, 0x76, 0xf0, 0x7d, 0x0d, 0xd6, 0x73, 0xf2,
	0x5e, 0xf6, 0x29, 0xf0, 0x47, 0x4b, 0xb8, 0x07, 0x8f, 0x95, 0x66, 0x72, 0x5a, 0x66, 0x81, 0x9d,
	0xea, 0xf7, 0xc9, 0x85, 0x44, 0xe6, 0x56, 0xaa, 0xbe, 0x16, 0x2d, 0x7c, 0x01, 0xfb, 0x4b, 0x70,
	0x46, 0x32, 0x25, 0x17, 0x06, 0x24, 0x1d, 0x1f, 0x91, 0x64, 0xd1, 0xc6, 0x7d, 0xd8, 0x5b, 0xee,
	0x32, 0xa0, 0xe4, 0x6d, 0x38, 0x2e, 0x8c, 0x2b, 0x72, 0x71, 0x03, 0xbb, 0x70, 0xd0, 0x50, 0xe6,
	0x1d, 0x12, 0xa9, 0xb5, 0xe1, 0x90, 0x18, 0xad, 0x29, 0xe1, 0xc0, 0x26, 0xf4, 0x4c, 0x96, 0x99,
	0xf7, 0xe4, 0xc4, 0x4d, 0xdc, 0x02, 0xb1, 0x64, 0xb4, 0x86, 0x4f, 0x60, 0xd7, 0x91, 0xb7, 0x46,
	0x7b, 0xaa, 0xf8, 0x4b, 0x8c, 0x5b, 0xb8, 0x03, 0xdb, 0x89, 0xd4, 0xa9, 0x4a, 0x25, 0x53, 0x70,
	0x74, 0x5c, 0x90, 0xe7, 0xf0, 0xce, 0x30, 0x89, 0x0e, 0xbe, 0x84, 0xa7, 0x8b, 0xea, 0x7f, 0xf0,
	0x6e, 0xe3, 0x36, 0xa0, 0x75, 0xc6, 0x1a, 0x5f, 0xad, 0xf3, 0xd7, 0x45, 0xdc, 0xc1, 0x07, 0xb0,
	0x31, 0x77, 0x94, 0xd6, 0x92, 0x4e, 0xc5, 0xdd, 0x2a, 0xbf, 0x15, 0x03, 0xcd, 0x61, 0xc0, 0x87,
	0xb0, 0x39, 0x7f, 0xf2, 0x5a, 0x5a, 0x3f, 0x30, 0x2c, 0xee, 0xe1, 0x6b, 0x78, 0xde, 0x44, 0xb1,
	0x42, 0xdc, 0x30, 0xd7, 0x71, 0x17, 0x1e, 0x2d, 0xe4, 0x2a, 0xb5, 0xef, 0x35, 0xf9, 0x8b, 0x8d,
	0x6a, 0xd5, 0x9e, 0x71, 0x09, 0x05, 0xca, 0x28, 0x61, 0x65, 0x74, 0x60, 0x95, 0x93, 0x29, 0x58,
	0xdc, 0xc7, 0x57, 0xf0, 0xcc, 0x91, 0x4c, 0x9b, 0xa8, 0x0b, 0xe7, 0x48, 0x57, 0x59, 0xe7, 0xb9,
	0x62, 0xa6, 0x34, 0x28, 0x9d, 0xd2, 0x07, 0xb1, 0x89, 0x87, 0x10, 0x2f, 0xce, 0xf0, 0x3f, 0x22,
	0x71, 0xb4, 0x75, 0xf5, 0x2b, 0x6a, 0x5d, 0xcd, 0xa2, 0xf6, 0x8f, 0x59, 0xd4, 0xfe, 0x39, 0x8b,
	0xda, 0xdf, 0x7e, 0x47, 0xad, 0x61, 0xa7, 0xfe, 0x63, 0x87, 0x7f, 0x02, 0x00, 0x00, 0xff, 0xff,
	0xa8, 0x8e, 0xcc, 0x63, 0xbc, 0x02, 0x00, 0x00,
}
