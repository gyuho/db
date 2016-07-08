// Code generated by protoc-gen-gogo.
// source: raft/raftpb/state.proto
// DO NOT EDIT!

package raftpb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// NODE_STATE represents the state of a node in a cluster.
type NODE_STATE int32

const (
	NODE_STATE_FOLLOWER  NODE_STATE = 0
	NODE_STATE_CANDIDATE NODE_STATE = 1
	NODE_STATE_LEADER    NODE_STATE = 2
)

var NODE_STATE_name = map[int32]string{
	0: "FOLLOWER",
	1: "CANDIDATE",
	2: "LEADER",
}
var NODE_STATE_value = map[string]int32{
	"FOLLOWER":  0,
	"CANDIDATE": 1,
	"LEADER":    2,
}

func (x NODE_STATE) String() string {
	return proto.EnumName(NODE_STATE_name, int32(x))
}
func (NODE_STATE) EnumDescriptor() ([]byte, []int) { return fileDescriptorState, []int{0} }

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
func (PROGRESS_STATE) EnumDescriptor() ([]byte, []int) { return fileDescriptorState, []int{1} }

// SNAPSHOT_STATUS represents the state of snapshot.
type SNAPSHOT_STATUS int32

const (
	SNAPSHOT_STATUS_FINISHED SNAPSHOT_STATUS = 0
	SNAPSHOT_STATUS_FAILED   SNAPSHOT_STATUS = 1
)

var SNAPSHOT_STATUS_name = map[int32]string{
	0: "FINISHED",
	1: "FAILED",
}
var SNAPSHOT_STATUS_value = map[string]int32{
	"FINISHED": 0,
	"FAILED":   1,
}

func (x SNAPSHOT_STATUS) String() string {
	return proto.EnumName(SNAPSHOT_STATUS_name, int32(x))
}
func (SNAPSHOT_STATUS) EnumDescriptor() ([]byte, []int) { return fileDescriptorState, []int{2} }

// SoftState provides state that is useful for logging and debugging.
// The state is volatile and does not need to be persisted to the WAL.
//
// (etcd raftpb.SoftState)
type SoftState struct {
	NodeState NODE_STATE `protobuf:"varint,1,opt,name=NodeState,json=nodeState,proto3,enum=raftpb.NODE_STATE" json:"NodeState,omitempty"`
	LeaderID  uint64     `protobuf:"varint,2,opt,name=LeaderID,json=leaderID,proto3" json:"LeaderID,omitempty"`
}

func (m *SoftState) Reset()                    { *m = SoftState{} }
func (m *SoftState) String() string            { return proto.CompactTextString(m) }
func (*SoftState) ProtoMessage()               {}
func (*SoftState) Descriptor() ([]byte, []int) { return fileDescriptorState, []int{0} }

func init() {
	proto.RegisterType((*SoftState)(nil), "raftpb.SoftState")
	proto.RegisterEnum("raftpb.NODE_STATE", NODE_STATE_name, NODE_STATE_value)
	proto.RegisterEnum("raftpb.PROGRESS_STATE", PROGRESS_STATE_name, PROGRESS_STATE_value)
	proto.RegisterEnum("raftpb.SNAPSHOT_STATUS", SNAPSHOT_STATUS_name, SNAPSHOT_STATUS_value)
}
func (m *SoftState) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *SoftState) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.NodeState != 0 {
		data[i] = 0x8
		i++
		i = encodeVarintState(data, i, uint64(m.NodeState))
	}
	if m.LeaderID != 0 {
		data[i] = 0x10
		i++
		i = encodeVarintState(data, i, uint64(m.LeaderID))
	}
	return i, nil
}

func encodeFixed64State(data []byte, offset int, v uint64) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	data[offset+4] = uint8(v >> 32)
	data[offset+5] = uint8(v >> 40)
	data[offset+6] = uint8(v >> 48)
	data[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32State(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintState(data []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		data[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	data[offset] = uint8(v)
	return offset + 1
}
func (m *SoftState) Size() (n int) {
	var l int
	_ = l
	if m.NodeState != 0 {
		n += 1 + sovState(uint64(m.NodeState))
	}
	if m.LeaderID != 0 {
		n += 1 + sovState(uint64(m.LeaderID))
	}
	return n
}

func sovState(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozState(x uint64) (n int) {
	return sovState(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *SoftState) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowState
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: SoftState: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SoftState: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field NodeState", wireType)
			}
			m.NodeState = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowState
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.NodeState |= (NODE_STATE(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field LeaderID", wireType)
			}
			m.LeaderID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowState
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.LeaderID |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipState(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthState
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipState(data []byte) (n int, err error) {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowState
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowState
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if data[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowState
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthState
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowState
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := data[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipState(data[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthState = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowState   = fmt.Errorf("proto: integer overflow")
)

var fileDescriptorState = []byte{
	// 297 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x44, 0x90, 0xd1, 0x4a, 0xf3, 0x30,
	0x18, 0x86, 0x9b, 0xf1, 0xff, 0xa3, 0xfd, 0x98, 0xb3, 0x04, 0xc1, 0xb1, 0x83, 0x32, 0x3c, 0x1a,
	0x13, 0x5b, 0x51, 0x04, 0x4f, 0xb3, 0x25, 0x73, 0x81, 0xd0, 0x94, 0xa4, 0x22, 0x1e, 0xc9, 0xea,
	0xba, 0x29, 0xa8, 0x19, 0x33, 0xbb, 0x17, 0x2f, 0x69, 0x87, 0x5e, 0x82, 0xce, 0x1b, 0x91, 0xa6,
	0x1b, 0x9e, 0x84, 0x3c, 0xc9, 0xf7, 0xbc, 0x2f, 0x7c, 0x70, 0xbc, 0x9a, 0xce, 0x6d, 0x52, 0x1d,
	0xcb, 0x22, 0x79, 0xb7, 0x53, 0x5b, 0xc6, 0xcb, 0x95, 0xb1, 0x06, 0x37, 0xeb, 0xb7, 0xee, 0xd9,
	0xe2, 0xd9, 0x3e, 0xad, 0x8b, 0xf8, 0xd1, 0xbc, 0x26, 0x0b, 0xb3, 0x30, 0x89, 0xfb, 0x2e, 0xd6,
	0x73, 0x47, 0x0e, 0xdc, 0xad, 0xd6, 0x4e, 0xee, 0x21, 0xd0, 0x66, 0x6e, 0x75, 0x95, 0x84, 0xcf,
	0x21, 0x48, 0xcd, 0xac, 0x74, 0xd0, 0x41, 0x3d, 0xd4, 0x6f, 0x5f, 0xe0, 0xb8, 0xce, 0x8d, 0x53,
	0x49, 0xd9, 0x83, 0xce, 0x49, 0xce, 0x54, 0xf0, 0xb6, 0x1f, 0xc2, 0x5d, 0xf0, 0x45, 0x39, 0x9d,
	0x95, 0x2b, 0x4e, 0x3b, 0x8d, 0x1e, 0xea, 0xff, 0x53, 0xfe, 0xcb, 0x8e, 0x07, 0x57, 0x00, 0x7f,
	0x12, 0x6e, 0x81, 0x3f, 0x96, 0x42, 0xc8, 0x3b, 0xa6, 0x42, 0x0f, 0x1f, 0x40, 0x30, 0x22, 0x29,
	0xe5, 0x94, 0xe4, 0x2c, 0x44, 0x18, 0xa0, 0x29, 0x18, 0xa1, 0x4c, 0x85, 0x8d, 0xc1, 0x35, 0xb4,
	0x33, 0x25, 0x6f, 0x14, 0xd3, 0x7a, 0xa7, 0x06, 0xf0, 0x3f, 0x53, 0x72, 0xc8, 0x6a, 0x4f, 0xb1,
	0x4c, 0xf0, 0x51, 0xed, 0xb5, 0xc0, 0xd7, 0x29, 0xc9, 0xf4, 0x44, 0xe6, 0x61, 0x63, 0x70, 0x0a,
	0x87, 0x7b, 0x72, 0xe6, 0xad, 0x76, 0xad, 0x3c, 0xe5, 0x7a, 0xc2, 0x68, 0xe8, 0x55, 0x35, 0x63,
	0xc2, 0x05, 0xa3, 0x21, 0x1a, 0x1e, 0x6d, 0xbe, 0x23, 0x6f, 0xb3, 0x8d, 0xd0, 0xe7, 0x36, 0x42,
	0x5f, 0xdb, 0x08, 0x7d, 0xfc, 0x44, 0x5e, 0xd1, 0x74, 0x5b, 0xb9, 0xfc, 0x0d, 0x00, 0x00, 0xff,
	0xff, 0x33, 0x6b, 0xe6, 0x85, 0x67, 0x01, 0x00, 0x00,
}