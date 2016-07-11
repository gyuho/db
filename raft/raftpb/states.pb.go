// Code generated by protoc-gen-gogo.
// source: raft/raftpb/states.proto
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
func (NODE_STATE) EnumDescriptor() ([]byte, []int) { return fileDescriptorStates, []int{0} }

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
func (PROGRESS_STATE) EnumDescriptor() ([]byte, []int) { return fileDescriptorStates, []int{1} }

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
func (*SoftState) Descriptor() ([]byte, []int) { return fileDescriptorStates, []int{0} }

// HardState is the current state of the Raft node.
// It must be stored in stable storage before messages are sent.
//
// (etcd raftpb.HardState)
type HardState struct {
	VotedFor       uint64 `protobuf:"varint,1,opt,name=VotedFor,json=votedFor,proto3" json:"VotedFor,omitempty"`
	CommittedIndex uint64 `protobuf:"varint,2,opt,name=CommittedIndex,json=committedIndex,proto3" json:"CommittedIndex,omitempty"`
	Term           uint64 `protobuf:"varint,3,opt,name=Term,json=term,proto3" json:"Term,omitempty"`
}

func (m *HardState) Reset()                    { *m = HardState{} }
func (m *HardState) String() string            { return proto.CompactTextString(m) }
func (*HardState) ProtoMessage()               {}
func (*HardState) Descriptor() ([]byte, []int) { return fileDescriptorStates, []int{1} }

func init() {
	proto.RegisterType((*SoftState)(nil), "raftpb.SoftState")
	proto.RegisterType((*HardState)(nil), "raftpb.HardState")
	proto.RegisterEnum("raftpb.NODE_STATE", NODE_STATE_name, NODE_STATE_value)
	proto.RegisterEnum("raftpb.PROGRESS_STATE", PROGRESS_STATE_name, PROGRESS_STATE_value)
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
		i = encodeVarintStates(data, i, uint64(m.NodeState))
	}
	if m.LeaderID != 0 {
		data[i] = 0x10
		i++
		i = encodeVarintStates(data, i, uint64(m.LeaderID))
	}
	return i, nil
}

func (m *HardState) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *HardState) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.VotedFor != 0 {
		data[i] = 0x8
		i++
		i = encodeVarintStates(data, i, uint64(m.VotedFor))
	}
	if m.CommittedIndex != 0 {
		data[i] = 0x10
		i++
		i = encodeVarintStates(data, i, uint64(m.CommittedIndex))
	}
	if m.Term != 0 {
		data[i] = 0x18
		i++
		i = encodeVarintStates(data, i, uint64(m.Term))
	}
	return i, nil
}

func encodeFixed64States(data []byte, offset int, v uint64) int {
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
func encodeFixed32States(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintStates(data []byte, offset int, v uint64) int {
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
		n += 1 + sovStates(uint64(m.NodeState))
	}
	if m.LeaderID != 0 {
		n += 1 + sovStates(uint64(m.LeaderID))
	}
	return n
}

func (m *HardState) Size() (n int) {
	var l int
	_ = l
	if m.VotedFor != 0 {
		n += 1 + sovStates(uint64(m.VotedFor))
	}
	if m.CommittedIndex != 0 {
		n += 1 + sovStates(uint64(m.CommittedIndex))
	}
	if m.Term != 0 {
		n += 1 + sovStates(uint64(m.Term))
	}
	return n
}

func sovStates(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozStates(x uint64) (n int) {
	return sovStates(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *SoftState) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowStates
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
					return ErrIntOverflowStates
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
					return ErrIntOverflowStates
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
			skippy, err := skipStates(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthStates
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
func (m *HardState) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowStates
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
			return fmt.Errorf("proto: HardState: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HardState: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field VotedFor", wireType)
			}
			m.VotedFor = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStates
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.VotedFor |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field CommittedIndex", wireType)
			}
			m.CommittedIndex = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStates
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.CommittedIndex |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Term", wireType)
			}
			m.Term = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStates
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.Term |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipStates(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthStates
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
func skipStates(data []byte) (n int, err error) {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowStates
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
					return 0, ErrIntOverflowStates
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
					return 0, ErrIntOverflowStates
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
				return 0, ErrInvalidLengthStates
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowStates
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
				next, err := skipStates(data[start:])
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
	ErrInvalidLengthStates = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowStates   = fmt.Errorf("proto: integer overflow")
)

var fileDescriptorStates = []byte{
	// 331 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x54, 0x90, 0xc1, 0x4a, 0xeb, 0x40,
	0x14, 0x86, 0x9b, 0xde, 0xdc, 0x92, 0x1c, 0x7a, 0x43, 0x18, 0xee, 0x22, 0x74, 0x11, 0x4a, 0x17,
	0x52, 0x0a, 0x26, 0xa2, 0x08, 0x6e, 0xd3, 0x66, 0x6a, 0x03, 0x21, 0x09, 0x93, 0xa0, 0xb8, 0x92,
	0xa4, 0x99, 0xd6, 0x82, 0xe9, 0x94, 0xe9, 0x54, 0x7c, 0x14, 0x1f, 0xa9, 0x4b, 0x1f, 0x41, 0xeb,
	0x8b, 0x48, 0x27, 0x51, 0x71, 0x33, 0xcc, 0x77, 0xce, 0xff, 0x7f, 0x0c, 0x03, 0x16, 0xcf, 0x17,
	0xc2, 0x3d, 0x1e, 0x9b, 0xc2, 0xdd, 0x8a, 0x5c, 0xd0, 0xad, 0xb3, 0xe1, 0x4c, 0x30, 0xd4, 0xa9,
	0x87, 0xbd, 0xd3, 0xe5, 0x4a, 0x3c, 0xec, 0x0a, 0x67, 0xce, 0x2a, 0x77, 0xc9, 0x96, 0xcc, 0x95,
	0xeb, 0x62, 0xb7, 0x90, 0x24, 0x41, 0xde, 0xea, 0xda, 0xe0, 0x0e, 0xf4, 0x94, 0x2d, 0x44, 0x7a,
	0x54, 0xa1, 0x33, 0xd0, 0x23, 0x56, 0x52, 0x09, 0x96, 0xd2, 0x57, 0x86, 0xc6, 0x39, 0x72, 0x6a,
	0xaf, 0x13, 0xc5, 0x3e, 0xbe, 0x4f, 0x33, 0x2f, 0xc3, 0x44, 0x5f, 0x7f, 0x85, 0x50, 0x0f, 0xb4,
	0x90, 0xe6, 0x25, 0xe5, 0x81, 0x6f, 0xb5, 0xfb, 0xca, 0x50, 0x25, 0xda, 0x63, 0xc3, 0x83, 0x39,
	0xe8, 0xb3, 0x9c, 0x97, 0xdf, 0xc1, 0x1b, 0x26, 0x68, 0x39, 0x65, 0x5c, 0x9a, 0x55, 0xa2, 0x3d,
	0x35, 0x8c, 0x4e, 0xc0, 0x98, 0xb0, 0xaa, 0x5a, 0x09, 0x41, 0xcb, 0x60, 0x5d, 0xd2, 0xe7, 0x46,
	0x65, 0xcc, 0x7f, 0x4d, 0x11, 0x02, 0x35, 0xa3, 0xbc, 0xb2, 0xfe, 0xc8, 0xad, 0x2a, 0x28, 0xaf,
	0x46, 0x97, 0x00, 0x3f, 0x2f, 0x43, 0x5d, 0xd0, 0xa6, 0x71, 0x18, 0xc6, 0xb7, 0x98, 0x98, 0x2d,
	0xf4, 0x0f, 0xf4, 0x89, 0x17, 0xf9, 0x81, 0xef, 0x65, 0xd8, 0x54, 0x10, 0x40, 0x27, 0xc4, 0x9e,
	0x8f, 0x89, 0xd9, 0x1e, 0x5d, 0x81, 0x91, 0x90, 0xf8, 0x9a, 0xe0, 0x34, 0x6d, 0xaa, 0x3a, 0xfc,
	0x4d, 0x48, 0x3c, 0xc6, 0x75, 0x8f, 0xe0, 0x24, 0x0c, 0x26, 0x75, 0xaf, 0x0b, 0x5a, 0x1a, 0x79,
	0x49, 0x3a, 0x8b, 0x33, 0xb3, 0x3d, 0xfe, 0xbf, 0x7f, 0xb7, 0x5b, 0xfb, 0x83, 0xad, 0xbc, 0x1e,
	0x6c, 0xe5, 0xed, 0x60, 0x2b, 0x2f, 0x1f, 0x76, 0xab, 0xe8, 0xc8, 0xdf, 0xbc, 0xf8, 0x0c, 0x00,
	0x00, 0xff, 0xff, 0xe8, 0xce, 0xec, 0xf6, 0xa0, 0x01, 0x00, 0x00,
}