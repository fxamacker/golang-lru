// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lru

import (
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// GroupCache is a thread-safe fixed size LRU cache.
type GroupCache[G comparable, K comparable, V any] struct {
	Cache[K, V]
}

// NewGroupCache creates an LRU with group of the given size.
func NewGroupCache[G comparable, K comparable, V any](
	size int,
	groupFromKey func(K) G,
) (*GroupCache[G, K, V], error) {
	return NewGroupCacheWithEvict[G, K, V](size, groupFromKey, nil)
}

// NewGroupCacheWithEvict constructs a fixed size cache with the given eviction
// callback.
func NewGroupCacheWithEvict[G comparable, K comparable, V any](
	size int,
	groupFromKey func(K) G,
	onEvicted func(key K, value V),
) (c *GroupCache[G, K, V], err error) {
	c = &GroupCache[G, K, V]{
		Cache: Cache[K, V]{
			onEvictedCB: onEvicted,
		},
	}
	if onEvicted != nil {
		c.initEvictBuffers()
		onEvicted = c.onEvicted
	}
	c.lru, err = simplelru.NewGroupLRU(size, groupFromKey, onEvicted)
	return
}

// RemoveGroup removes all keys associated with given group from the cache.
func (c *GroupCache[G, K, V]) RemoveGroup(group G) (counter int) {
	var ks []K
	var vs []V
	c.lock.Lock()
	counter = c.lru.(*simplelru.GroupLRU[G, K, V]).RemoveGroup(group)
	if c.onEvictedCB != nil && counter > 0 {
		ks, vs = c.evictedKeys, c.evictedVals
		c.initEvictBuffers()
	}
	c.lock.Unlock()
	if c.onEvictedCB != nil && counter > 0 {
		for i := 0; i < len(ks); i++ {
			c.onEvictedCB(ks[i], vs[i])
		}
	}
	return
}

// RemoveGroups removes all keys associated with given groups from the cache.
func (c *GroupCache[G, K, V]) RemoveGroups(groups []G) (counter int) {
	var ks []K
	var vs []V
	c.lock.Lock()
	counter = c.lru.(*simplelru.GroupLRU[G, K, V]).RemoveGroups(groups)
	if c.onEvictedCB != nil && counter > 0 {
		ks, vs = c.evictedKeys, c.evictedVals
		c.initEvictBuffers()
	}
	c.lock.Unlock()
	if c.onEvictedCB != nil && counter > 0 {
		for i := 0; i < len(ks); i++ {
			c.onEvictedCB(ks[i], vs[i])
		}
	}
	return
}
