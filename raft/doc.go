// Package raft implements the Raft consensus algorithm https://github.com/ongardie/dissertation.
//
// Main goal is to decompose core implementation into independent components, which makes code
// easier to test and more determinstic.
//
// Reference implementation is https://github.com/coreos/etcd/tree/master/raft.
//
package raft

/*
???
Raft(short) 8

Raft handles this by having each leader commit a blank no-op entry into the log at the start of its
term. Second, a leader must check whether it has been deposed before processing a read-only request (its information may be stale if a more recent leader has been elected).
Raft handles this by having the leader exchange heartbeat messages with a majority of the cluster before responding to read-only requests. Alternatively, the leader
could rely on the heartbeat mechanism to provide a form
of lease [9], but this would rely on timing for safety (it
assumes bounded clock skew).

???
Raft 4.2.3 p.42

???
Raft 6.3 Implementing linearizable semantics
unique client id
*/
