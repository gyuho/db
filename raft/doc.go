// Package raft implements the Raft consensus algorithm https://github.com/ongardie/dissertation.
//
// Main goal is to decompose core implementation into independent components, which makes code
// easier to test and more determinstic.
//
// Reference implementation is https://github.com/coreos/etcd/tree/master/raft.
//
package raft
