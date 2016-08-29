package rafthttp

import "github.com/gyuho/db/pkg/xlog"

func init() {
	logger.SetMaxLogLevel(xlog.CRITICAL)
}
