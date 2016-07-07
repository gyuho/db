// Package raft implements the Raft consensus algorithm
// (https://github.com/ongardie/dissertation).
//
// Reference implementation is https://github.com/coreos/etcd/tree/master/raft.
//
// The main goal is to decouple the core algorithms from transportation and storage layer.
// Simple interface makes Raft more portable for different use cases.
//
package raft
