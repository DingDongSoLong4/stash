package db

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/corona10/goimagehash"

	lru "github.com/hashicorp/golang-lru"
)

// size of the regex LRU cache in elements.
// A small number number was chosen because it's most likely use is for a
// single query - this function gets called for every row in the (filtered)
// results. It's likely to only need no more than 1 or 2 in any given query.
// After that point, it's just sitting in the cache and is unlikely to be used
// again.
const regexCacheSize = 10

var regexCache *lru.Cache

func init() {
	regexCache, _ = lru.New(regexCacheSize)
}

func regexpGet(re string) (*regexp.Regexp, error) {
	entry, ok := regexCache.Get(re)
	var ret *regexp.Regexp

	if !ok {
		var err error
		ret, err = regexp.Compile(re)
		if err != nil {
			return nil, err
		}
		regexCache.Add(re, ret)
	} else {
		ret = entry.(*regexp.Regexp)
	}

	return ret, nil
}

// regexpFn is registered as an SQLite function as "regexp"
// It uses an LRU cache to cache recent regex patterns to reduce CPU load over
// identical patterns.
func regexpFn(re, s string) (bool, error) {
	compiled, err := regexpGet(re)
	if err != nil {
		return false, err
	}

	return compiled.MatchString(s), nil
}

func durationToTinyIntFn(str string) (int64, error) {
	splits := strings.Split(str, ":")

	if len(splits) > 3 {
		return 0, nil
	}

	seconds := 0
	factor := 1
	for len(splits) > 0 {
		// pop the last split
		var thisSplit string
		thisSplit, splits = splits[len(splits)-1], splits[:len(splits)-1]

		thisInt, err := strconv.Atoi(thisSplit)
		if err != nil {
			return 0, nil
		}

		seconds += factor * thisInt
		factor *= 60
	}

	return int64(seconds), nil
}

func basenameFn(str string) (string, error) {
	return filepath.Base(str), nil
}

func phashDistanceFn(phashStr1 string, phashStr2 string) (int, error) {
	phash1, err := strconv.ParseUint(phashStr1, 16, 64)
	if err != nil {
		return 0, err
	}
	phash2, err := strconv.ParseUint(phashStr2, 16, 64)
	if err != nil {
		return 0, err
	}

	hash1 := goimagehash.NewImageHash(phash1, goimagehash.PHash)
	hash2 := goimagehash.NewImageHash(phash2, goimagehash.PHash)
	distance, _ := hash1.Distance(hash2)
	return distance, nil
}
