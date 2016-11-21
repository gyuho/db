package mvcc

import (
	"sync"

	"github.com/google/btree"
)

type treeIndex struct {
	sync.RWMutex
	tree *btree.BTree
}
