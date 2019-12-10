package http

import (
	"github.com/segmentio/ksuid"
	"time"
)

//  queue of failed operations to retry as a circular buffer with a set of data structures that look something like this:

type retryQueue struct {
	buckets       [][]retryItem
	currentTime   time.Time
	currentOffset int
}

type retryItem struct {
	id   ksuid.KSUID
	time time.Time
}
