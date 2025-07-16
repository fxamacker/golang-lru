// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package simplelru

// GroupFromKey is used to get group from key.
// This function is called when a cache entry is stored, removed, or evicted.
type GroupFromKey[G, K comparable] func(key K) G

// GroupLRU implements a non-thread safe fixed size LRU cache
type GroupLRU[G comparable, K comparable, V any] struct {
	*LRU[K, V]
	groupFromKey GroupFromKey[G, K]
	groups       map[G]map[K]struct{}
}

// NewGroupLRU constructs an LRU of the given size
func NewGroupLRU[G comparable, K comparable, V any](
	size int,
	groupFromKey GroupFromKey[G, K],
	onEvict EvictCallback[K, V],
) (*GroupLRU[G, K, V], error) {
	lru, err := NewLRU(size, onEvict)
	if err != nil {
		return nil, err
	}
	c := &GroupLRU[G, K, V]{
		LRU:          lru,
		groupFromKey: groupFromKey,
		groups:       make(map[G]map[K]struct{}),
	}
	return c, nil
}

// Purge is used to completely clear the cache.
func (c *GroupLRU[G, K, V]) Purge() {
	c.LRU.Purge()
	c.groups = make(map[G]map[K]struct{})
}

// Add adds a value to the cache.  Returns true if an eviction occurred.
func (c *GroupLRU[G, K, V]) Add(key K, value V) (evicted bool) {
	// Add key and value to LRU cache
	evictedKey, evicted := c.LRU.add(key, value)

	// Add new key to groups.
	c.addKeyToGroups(key)

	if evicted {
		// Remove evicted key from groups.
		c.removeKeyFromGroups(evictedKey)
	}
	return evicted
}

// Remove removes the provided key from the cache, returning if the
// key was contained.
func (c *GroupLRU[G, K, V]) Remove(key K) (present bool) {
	present = c.LRU.Remove(key)
	c.removeKeyFromGroups(key)
	return
}

// RemoveGroup removes all keys associated with the given group and returns number of keys removed.
func (c *GroupLRU[G, K, V]) RemoveGroup(group G) (count int) {
	keys := c.groups[group]
	count = len(keys)
	for key := range keys {
		c.LRU.Remove(key)
	}
	delete(c.groups, group)
	return
}

// RemoveGroups removes all keys associated with the given groups and returns number of keys removed.
func (c *GroupLRU[G, K, V]) RemoveGroups(groups []G) (count int) {
	for _, group := range groups {
		count += c.RemoveGroup(group)
	}
	return
}

// RemoveOldest removes the oldest item from the cache.
func (c *GroupLRU[G, K, V]) RemoveOldest() (key K, value V, ok bool) {
	key, value, ok = c.LRU.RemoveOldest()
	c.removeKeyFromGroups(key)
	return
}

// Resize changes the cache size.
func (c *GroupLRU[G, K, V]) Resize(size int) (evicted int) {
	diff := c.Len() - size
	if diff < 0 {
		diff = 0
	}
	for i := 0; i < diff; i++ {
		evictedKey, evicted := c.removeOldest()
		if evicted {
			c.removeKeyFromGroups(evictedKey)
		}
	}
	c.size = size
	return diff
}

func (c *GroupLRU[G, K, V]) addKeyToGroups(key K) {
	group := c.groupFromKey(key)

	keys := c.groups[group]
	if keys == nil {
		keys = make(map[K]struct{})
		c.groups[group] = keys
	}

	keys[key] = struct{}{}
}

func (c *GroupLRU[G, K, V]) removeKeyFromGroups(key K) {
	group := c.groupFromKey(key)

	delete(c.groups[group], key)

	if len(c.groups[group]) == 0 {
		delete(c.groups, group)
	}
}
