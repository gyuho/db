// Code generated by protoc-gen-gogo.
// source: raft/raftpb/soft_state.proto
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
func (*SoftState) Descriptor() ([]byte, []int) { return fileDescriptorSoftState, []int{0} }

func init() {
	proto.RegisterType((*SoftState)(nil), "raftpb.SoftState")
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
		i = encodeVarintSoftState(data, i, uint64(m.NodeState))
	}
	if m.LeaderID != 0 {
		data[i] = 0x10
		i++
		i = encodeVarintSoftState(data, i, uint64(m.LeaderID))
	}
	return i, nil
}

func encodeFixed64SoftState(data []byte, offset int, v uint64) int {
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
func encodeFixed32SoftState(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintSoftState(data []byte, offset int, v uint64) int {
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
		n += 1 + sovSoftState(uint64(m.NodeState))
	}
	if m.LeaderID != 0 {
		n += 1 + sovSoftState(uint64(m.LeaderID))
	}
	return n
}

func sovSoftState(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozSoftState(x uint64) (n int) {
	return sovSoftState(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *SoftState) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSoftState
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
					return ErrIntOverflowSoftState
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
					return ErrIntOverflowSoftState
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
			skippy, err := skipSoftState(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthSoftState
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
func skipSoftState(data []byte) (n int, err error) {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowSoftState
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
					return 0, ErrIntOverflowSoftState
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
					return 0, ErrIntOverflowSoftState
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
				return 0, ErrInvalidLengthSoftState
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowSoftState
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
				next, err := skipSoftState(data[start:])
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
	ErrInvalidLengthSoftState = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowSoftState   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("raft/raftpb/soft_state.proto", fileDescriptorSoftState) }

var fileDescriptorSoftState = []byte{
	// 194 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xe2, 0x92, 0x29, 0x4a, 0x4c, 0x2b,
	0xd1, 0x07, 0x11, 0x05, 0x49, 0xfa, 0xc5, 0xf9, 0x69, 0x25, 0xf1, 0xc5, 0x25, 0x89, 0x25, 0xa9,
	0x7a, 0x05, 0x45, 0xf9, 0x25, 0xf9, 0x42, 0x6c, 0x10, 0x09, 0x29, 0xdd, 0xf4, 0xcc, 0x92, 0x8c,
	0xd2, 0x24, 0xbd, 0xe4, 0xfc, 0x5c, 0xfd, 0xf4, 0xfc, 0xf4, 0x7c, 0x7d, 0xb0, 0x74, 0x52, 0x69,
	0x1a, 0x98, 0x07, 0xe6, 0x80, 0x59, 0x10, 0x6d, 0x52, 0x28, 0x86, 0xe6, 0xe5, 0xa7, 0xa4, 0x22,
	0x1b, 0xaa, 0x14, 0xc9, 0xc5, 0x19, 0x9c, 0x9f, 0x56, 0x12, 0x0c, 0x12, 0x12, 0x32, 0xe0, 0xe2,
	0xf4, 0xcb, 0x4f, 0x49, 0x05, 0x73, 0x24, 0x18, 0x15, 0x18, 0x35, 0xf8, 0x8c, 0x84, 0xf4, 0x20,
	0x3a, 0xf5, 0xfc, 0xfc, 0x5d, 0x5c, 0xe3, 0x83, 0x43, 0x1c, 0x43, 0x5c, 0x83, 0x38, 0xf3, 0x60,
	0x8a, 0x84, 0xa4, 0xb8, 0x38, 0x7c, 0x52, 0x13, 0x53, 0x52, 0x8b, 0x3c, 0x5d, 0x24, 0x98, 0x14,
	0x18, 0x35, 0x58, 0x82, 0x38, 0x72, 0xa0, 0x7c, 0x27, 0x91, 0x13, 0x0f, 0xe5, 0x18, 0x4e, 0x3c,
	0x92, 0x63, 0xbc, 0xf0, 0x48, 0x8e, 0xf1, 0xc1, 0x23, 0x39, 0xc6, 0x19, 0x8f, 0xe5, 0x18, 0x92,
	0xd8, 0xc0, 0xf6, 0x1a, 0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0xe6, 0x6e, 0xc2, 0xa9, 0xec, 0x00,
	0x00, 0x00,
}
