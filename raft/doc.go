// Package raft implements the Raft consensus algorithm https://github.com/ongardie/dissertation.
//
// The main goal is to decouple the core algorithms from transportation and storage layer.
// Simple interface makes Raft more portable for different use cases, easier to test,
// and more determinstic.
//
// Reference implementation is https://github.com/coreos/etcd/tree/master/raft.
//
package raft
