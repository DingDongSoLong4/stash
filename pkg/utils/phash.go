package utils

import (
	"math"

	"github.com/corona10/goimagehash"
	"github.com/stashapp/stash/pkg/sliceutil/intslice"
)

type Phash struct {
	SceneID   int     `db:"id"`
	Hash      int64   `db:"phash"`
	Duration  float64 `db:"duration"`
	Neighbors []int
	Bucket    int
}

func FindDuplicates(hashes []*Phash, distance int, durationDiff float64) [][]int {
	for i, scene := range hashes {
		sceneHash := goimagehash.NewImageHash(uint64(scene.Hash), goimagehash.PHash)
		for j, neighbor := range hashes {
			if i != j && scene.SceneID != neighbor.SceneID {
				neighbourDurationDistance := 0.
				if scene.Duration > 0 && neighbor.Duration > 0 {
					neighbourDurationDistance = math.Abs(scene.Duration - neighbor.Duration)
				}
				if (neighbourDurationDistance <= durationDiff) || (durationDiff < 0) {
					neighborHash := goimagehash.NewImageHash(uint64(neighbor.Hash), goimagehash.PHash)
					neighborDistance, _ := sceneHash.Distance(neighborHash)
					if neighborDistance <= distance {
						scene.Neighbors = append(scene.Neighbors, j)
					}
				}
			}
		}
	}

	var buckets [][]int
	for _, scene := range hashes {
		if len(scene.Neighbors) > 0 && scene.Bucket == -1 {
			bucket := len(buckets)
			scenes := []int{scene.SceneID}
			scene.Bucket = bucket
			findNeighbors(bucket, scene.Neighbors, hashes, &scenes)

			if len(scenes) > 1 {
				buckets = append(buckets, scenes)
			}
		}
	}

	return buckets
}

func findNeighbors(bucket int, neighbors []int, hashes []*Phash, scenes *[]int) {
	for _, id := range neighbors {
		hash := hashes[id]
		if hash.Bucket == -1 {
			hash.Bucket = bucket
			*scenes = intslice.IntAppendUnique(*scenes, hash.SceneID)
			findNeighbors(bucket, hash.Neighbors, hashes, scenes)
		}
	}
}
