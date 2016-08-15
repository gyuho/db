package raftpb

import (
	"encoding/binary"
	"io"
)

// MessageBinaryEncoder encodes(marshals) Message in binary format.
//
// (etcd rafthttp.messageEncoder)
type MessageBinaryEncoder struct {
	w io.Writer
}

// NewMessageBinaryEncoder returns a new MessageBinaryEncoder with given writer.
func NewMessageBinaryEncoder(w io.Writer) *MessageBinaryEncoder {
	return &MessageBinaryEncoder{w: w}
}

// Encode encodes Message to writer.
func (enc *MessageBinaryEncoder) Encode(msg *Message) error {
	if err := binary.Write(enc.w, binary.BigEndian, uint64(msg.Size())); err != nil {
		return err
	}

	bts, err := msg.Marshal()
	if err != nil {
		return err
	}

	_, err = enc.w.Write(bts)
	return err
}

// MessageBinaryDecoder decodes(unmarshals) bytes to Message.
//
// (etcd rafthttp.messageEncoder)
type MessageBinaryDecoder struct {
	r io.Reader
}

// NewMessageBinaryDecoder returns a new MessageBinaryDecoder with given reader.
func NewMessageBinaryDecoder(r io.Reader) *MessageBinaryDecoder {
	return &MessageBinaryDecoder{r: r}
}

// Decode decodes Message from reader.
func (dec *MessageBinaryDecoder) Decode() (Message, error) {
	var bNum uint64
	if err := binary.Read(dec.r, binary.BigEndian, &bNum); err != nil {
		return Message{}, err
	}

	src := make([]byte, int(bNum))
	if _, err := io.ReadFull(dec.r, src); err != nil {
		return Message{}, err
	}

	var msg Message
	err := msg.Unmarshal(src)
	return msg, err
}
