package raft

import "github.com/gyuho/db/pkg/xlog"

func init() {
	raftLogger.SetLogger(xlog.NewLogger("raft", xlog.CRITICAL))
}
