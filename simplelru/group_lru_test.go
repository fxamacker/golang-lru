// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package simplelru

import (
	"reflect"
	"testing"
)

var groupFromKey = func(key int) string {
	if key%2 == 0 {
		return "even"
	} else {
		return "odd"
	}
}

//gocyclo:ignore
func TestGroupLRU(t *testing.T) {
	evictCounter := 0
	onEvicted := func(k int, v int) {
		if k != v {
			t.Fatalf("Evict values not equal (%v!=%v)", k, v)
		}
		evictCounter++
	}
	l, err := NewGroupLRU(128, groupFromKey, onEvicted)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for i := 0; i < 256; i++ {
		l.Add(i, i)
		testGroupInSyncWithMainCache(t, l)
	}
	if l.Len() != 128 {
		t.Fatalf("bad len: %v", l.Len())
	}
	if l.Cap() != 128 {
		t.Fatalf("expect %d, but %d", 128, l.Cap())
	}
	if len(l.groups) != 2 {
		t.Fatalf("expect %d groups, but %d groups", 2, len(l.groups))
	}

	if evictCounter != 128 {
		t.Fatalf("bad evict count: %v", evictCounter)
	}

	for i, k := range l.Keys() {
		if v, ok := l.Get(k); !ok || v != k || v != i+128 {
			t.Fatalf("bad key: %v", k)
		}
	}
	for i, v := range l.Values() {
		if v != i+128 {
			t.Fatalf("bad value: %v", v)
		}
	}
	for i := 0; i < 128; i++ {
		if _, ok := l.Get(i); ok {
			t.Fatalf("should be evicted")
		}
	}
	for i := 128; i < 256; i++ {
		if _, ok := l.Get(i); !ok {
			t.Fatalf("should not be evicted")
		}
	}
	for i := 128; i < 192; i++ {
		if ok := l.Remove(i); !ok {
			t.Fatalf("should be contained")
		}
		if ok := l.Remove(i); ok {
			t.Fatalf("should not be contained")
		}
		if _, ok := l.Get(i); ok {
			t.Fatalf("should be deleted")
		}
		testGroupInSyncWithMainCache(t, l)
	}

	l.Get(192) // expect 192 to be last key in l.Keys()

	for i, k := range l.Keys() {
		if (i < 63 && k != i+193) || (i == 63 && k != 192) {
			t.Fatalf("out of order key: %v", k)
		}
	}

	oddGroupLen := len(l.groups["odd"])
	removedCount := l.RemoveGroup("odd")
	if oddGroupLen != removedCount {
		t.Fatalf("expect %d removed items from group %s, got %d", oddGroupLen, "odd", removedCount)
	}
	if len(l.groups) != 1 {
		t.Fatalf("expect %d groups, got %d", 1, len(l.groups))
	}

	testGroupInSyncWithMainCache(t, l)

	l.Purge()
	if l.Len() != 0 {
		t.Fatalf("bad len: %v", l.Len())
	}
	if len(l.groups) != 0 {
		t.Fatalf("bad group len: %d", len(l.groups))
	}
	if _, ok := l.Get(200); ok {
		t.Fatalf("should contain nothing")
	}
}

func TestGroupLRU_GetOldest_RemoveOldest(t *testing.T) {
	l, err := NewGroupLRU[string, int, int](128, groupFromKey, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for i := 0; i < 256; i++ {
		l.Add(i, i)
		testGroupInSyncWithMainCache(t, l)
	}
	k, _, ok := l.GetOldest()
	if !ok {
		t.Fatalf("missing")
	}
	if k != 128 {
		t.Fatalf("bad: %v", k)
	}

	k, _, ok = l.RemoveOldest()
	if !ok {
		t.Fatalf("missing")
	}
	if k != 128 {
		t.Fatalf("bad: %v", k)
	}
	testGroupInSyncWithMainCache(t, l)

	k, _, ok = l.RemoveOldest()
	if !ok {
		t.Fatalf("missing")
	}
	if k != 129 {
		t.Fatalf("bad: %v", k)
	}
	testGroupInSyncWithMainCache(t, l)
}

// Test that Add returns true/false if an eviction occurred
func TestGroupLRU_Add(t *testing.T) {
	evictCounter := 0
	onEvicted := func(k int, v int) {
		evictCounter++
	}

	l, err := NewGroupLRU(1, groupFromKey, onEvicted)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if l.Add(1, 1) == true || evictCounter != 0 {
		t.Errorf("should not have an eviction")
	}
	testGroupInSyncWithMainCache(t, l)
	if l.Add(2, 2) == false || evictCounter != 1 {
		t.Errorf("should have an eviction")
	}
	testGroupInSyncWithMainCache(t, l)
}

// Test that Contains doesn't update recent-ness
func TestGroupLRU_Contains(t *testing.T) {
	l, err := NewGroupLRU[string, int, int](2, groupFromKey, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	l.Add(1, 1)
	l.Add(2, 2)
	if !l.Contains(1) {
		t.Errorf("1 should be contained")
	}

	l.Add(3, 3)
	if l.Contains(1) {
		t.Errorf("Contains should not have updated recent-ness of 1")
	}

	testGroupInSyncWithMainCache(t, l)
}

// Test that Resize can upsize and downsize
func TestGroupLRU_Resize(t *testing.T) {
	onEvictCounter := 0
	onEvicted := func(k int, v int) {
		onEvictCounter++
	}
	l, err := NewGroupLRU(2, groupFromKey, onEvicted)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Downsize
	l.Add(1, 1)
	l.Add(2, 2)
	testGroupInSyncWithMainCache(t, l)

	evicted := l.Resize(1)
	if evicted != 1 {
		t.Errorf("1 element should have been evicted: %v", evicted)
	}
	if onEvictCounter != 1 {
		t.Errorf("onEvicted should have been called 1 time: %v", onEvictCounter)
	}
	testGroupInSyncWithMainCache(t, l)

	l.Add(3, 3)
	if l.Contains(1) {
		t.Errorf("Element 1 should have been evicted")
	}
	testGroupInSyncWithMainCache(t, l)

	// Upsize
	evicted = l.Resize(2)
	if evicted != 0 {
		t.Errorf("0 elements should have been evicted: %v", evicted)
	}
	testGroupInSyncWithMainCache(t, l)

	l.Add(4, 4)
	if !l.Contains(3) || !l.Contains(4) {
		t.Errorf("Cache should have contained 2 elements")
	}
	testGroupInSyncWithMainCache(t, l)
}

func (c *GroupLRU[G, K, V]) wantKeys(t *testing.T, want []K) {
	t.Helper()
	got := c.Keys()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrong keys got: %v, want: %v ", got, want)
	}
}

func TestGroupCache_EvictionSameKey(t *testing.T) {
	var evictedKeys []int

	cache, _ := NewGroupLRU(
		2,
		groupFromKey,
		func(key int, _ struct{}) {
			evictedKeys = append(evictedKeys, key)
		})

	if evicted := cache.Add(1, struct{}{}); evicted {
		t.Error("First 1: got unexpected eviction")
	}
	cache.wantKeys(t, []int{1})
	testGroupInSyncWithMainCache(t, cache)

	if evicted := cache.Add(2, struct{}{}); evicted {
		t.Error("2: got unexpected eviction")
	}
	cache.wantKeys(t, []int{1, 2})
	testGroupInSyncWithMainCache(t, cache)

	if evicted := cache.Add(1, struct{}{}); evicted {
		t.Error("Second 1: got unexpected eviction")
	}
	cache.wantKeys(t, []int{2, 1})
	testGroupInSyncWithMainCache(t, cache)

	if evicted := cache.Add(3, struct{}{}); !evicted {
		t.Error("3: did not get expected eviction")
	}
	cache.wantKeys(t, []int{1, 3})
	testGroupInSyncWithMainCache(t, cache)

	want := []int{2}
	if !reflect.DeepEqual(evictedKeys, want) {
		t.Errorf("evictedKeys got: %v want: %v", evictedKeys, want)
	}
}

func TestGroupCache_RemoveGroup(t *testing.T) {
	const cacheSize = 128

	cases := []struct {
		name             string
		elementsToAdd    []int
		groupsToRemove   []string
		wantEvictEntries map[int]int
	}{
		{
			name:             "remove group from empty cache",
			groupsToRemove:   []string{"odd"},
			wantEvictEntries: map[int]int{},
		},
		{
			name:             "remove non-existent group from cache",
			elementsToAdd:    []int{0},
			groupsToRemove:   []string{"odd"},
			wantEvictEntries: map[int]int{},
		},
		{
			name:             "remove group with 1 key from cache",
			elementsToAdd:    []int{0, 1, 2},
			groupsToRemove:   []string{"odd"},
			wantEvictEntries: map[int]int{1: 1},
		},
		{
			name:             "remove group with multiple keys from cache",
			elementsToAdd:    []int{0, 1, 2},
			groupsToRemove:   []string{"even"},
			wantEvictEntries: map[int]int{0: 0, 2: 2},
		},
		{
			name:           "remove group with all keys from cache",
			elementsToAdd:  []int{0, 2, 4, 6, 8},
			groupsToRemove: []string{"even"},
			wantEvictEntries: map[int]int{
				0: 0,
				2: 2,
				4: 4,
				6: 6,
				8: 8,
			},
		},
		{
			name:           "remove all groups",
			elementsToAdd:  []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			groupsToRemove: []string{"even", "odd"},
			wantEvictEntries: map[int]int{
				0: 0,
				2: 2,
				4: 4,
				6: 6,
				8: 8,
				1: 1,
				3: 3,
				5: 5,
				7: 7,
				9: 9,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			evictEntries := make(map[int]int)
			onEvicted := func(k int, v int) {
				evictEntries[k] = v
			}

			l, err := NewGroupLRU(cacheSize, groupFromKey, onEvicted)
			if err != nil {
				t.Fatalf("failed to create GroupLRU: %v", err)
			}

			for _, ele := range c.elementsToAdd {
				evicted := l.Add(ele, ele)
				if evicted {
					t.Error("expect no eviction, got one")
				}
				testGroupInSyncWithMainCache(t, l)
			}

			removeCounter := l.RemoveGroups(c.groupsToRemove)

			testGroupInSyncWithMainCache(t, l)

			if removeCounter != len(c.wantEvictEntries) {
				t.Errorf("expect %d removeCount, got %d", len(c.wantEvictEntries), removeCounter)
			}

			if !reflect.DeepEqual(evictEntries, c.wantEvictEntries) {
				t.Errorf("expect evictEntries %v, got %v", c.wantEvictEntries, evictEntries)
			}

			if l.Len() != len(c.elementsToAdd)-len(c.wantEvictEntries) {
				t.Errorf("expect lru len %d,  got %d", len(c.elementsToAdd)-len(c.wantEvictEntries), l.Len())
			}

			for _, ele := range c.elementsToAdd {
				if _, evicted := c.wantEvictEntries[ele]; evicted {
					if l.Contains(ele) {
						t.Errorf("%d should not exist", ele)
					}
				} else {
					v, ok := l.Peek(ele)
					if !ok {
						t.Errorf("%d should exist", ele)
					} else if v != ele {
						t.Errorf("%d should be set to %d", v, ele)
					}
				}
			}
		})
	}
}

func testGroupInSyncWithMainCache[G comparable, K comparable, V any](t *testing.T, c *GroupLRU[G, K, V]) {
	// Test group cache size
	groupCacheSize := 0
	for group, keys := range c.groups {
		groupCacheSize += len(keys)
		if len(keys) == 0 {
			t.Fatalf("found empty group %v", group)
		}
	}
	if c.Len() != groupCacheSize {
		t.Fatalf("cache size %d != group cache size %d", c.Len(), groupCacheSize)
	}

	// Test group cache content
	for _, key := range c.Keys() {
		group := c.groupFromKey(key)
		if _, exists := c.groups[group][key]; !exists {
			t.Fatalf("key %v doesn't exist in group %v", key, group)
		}
	}
}
