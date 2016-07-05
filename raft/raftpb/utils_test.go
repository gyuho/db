package raftpb

import (
	"fmt"
	"testing"
)

func Test_DescribeEntry(t *testing.T) {
	entry := Entry{
		Type:  ENTRY_TYPE_NORMAL,
		Index: 2,
		Term:  1,
		Data:  []byte("hello"),
	}
	fmt.Println(DescribeEntry(entry))
}

func Test_DescribeMessage(t *testing.T) {
	entry := Entry{
		Type:  ENTRY_TYPE_NORMAL,
		Index: 2,
		Term:  1,
		Data:  []byte("hello"),
	}
	msg := Message{
		Type:     MESSAGE_TYPE_APPEND_RESPONSE,
		From:     7777,
		To:       9999,
		LogIndex: 10,
		LogTerm:  5,
		Entries:  []Entry{entry},
	}
	fmt.Println(DescribeMessage(msg))
}
