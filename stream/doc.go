// Package stream provides lazy, ordered, sequential composition over Go
// iterators.
//
// A Stream is a non-caching sequence descriptor. The zero value is empty, and
// reusable or single-use behavior is inherited from the source. Shuttle starts
// no worker goroutines, stores no hidden errors, and propagates downstream early
// termination to conforming iterator sources.
package stream
