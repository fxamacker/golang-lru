// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lru

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func BenchmarkGroupCacheRemoveGroups(b *testing.B) {

	type identifier [32]byte
	type twoIdentifier [64]byte

	const keyCountPerGroup = 5

	groupFromKey := func(key twoIdentifier) identifier {
		var group identifier
		copy(group[:], key[:])
		return group
	}

	benchmarks := []struct {
		cacheSize   int
		removeCount int
	}{
		{cacheSize: 1_000, removeCount: 25},
		{cacheSize: 2_000, removeCount: 25},
		{cacheSize: 3_000, removeCount: 25},
		{cacheSize: 4_000, removeCount: 25},
		{cacheSize: 5_000, removeCount: 25},
		{cacheSize: 6_000, removeCount: 25},
		{cacheSize: 7_000, removeCount: 25},
		{cacheSize: 8_000, removeCount: 25},
		{cacheSize: 9_000, removeCount: 25},
		{cacheSize: 10_000, removeCount: 25},
		{cacheSize: 20_000, removeCount: 25},
	}

	for _, bm := range benchmarks {
		name := fmt.Sprintf("cache size %d, remove count %d", bm.cacheSize, bm.removeCount)
		b.Run(name, func(b *testing.B) {
			groupCount := bm.cacheSize/keyCountPerGroup + 1

			groupIDs := make([]identifier, 0, groupCount)
			keyIDs := make([]twoIdentifier, 0, groupCount*keyCountPerGroup)
			for i := 0; i < groupCount; i++ {
				var id identifier
				_, _ = rand.Read(id[:])

				groupIDs = append(groupIDs, id)

				for j := 0; j < keyCountPerGroup; j++ {
					var twoID twoIdentifier
					_, _ = rand.Read(twoID[:])
					copy(twoID[:], id[:])

					keyIDs = append(keyIDs, twoID)
				}
			}

			removeGroupCount := bm.removeCount / keyCountPerGroup

			removeGroups := make([]identifier, removeGroupCount)
			copy(removeGroups, groupIDs[len(groupIDs)-removeGroupCount:])

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				b.StopTimer()

				cache, _ := NewGroupCache[identifier, twoIdentifier, struct{}](bm.cacheSize, groupFromKey)

				for _, key := range keyIDs {
					cache.Add(key, struct{}{})
				}

				b.StartTimer()

				cache.RemoveGroups(removeGroups)
			}
		})
	}
}
