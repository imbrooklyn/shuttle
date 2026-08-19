package stream_test

import (
	"slices"
	"sync/atomic"
	"testing"

	"github.com/imbrooklyn/shuttle/stream"
)

func TestZipLengthsLazinessAndConsumption(t *testing.T) {
	tests := []struct {
		name                        string
		left, right                 []int
		want                        []stream.Pair[int, int]
		leftConsumed, rightConsumed int64
		rightStarted                bool
	}{
		{name: "left empty", right: []int{10}, rightStarted: false},
		{name: "right empty", left: []int{1, 2}, leftConsumed: 1, rightStarted: true},
		{name: "left shorter", left: []int{1, 2}, right: []int{10, 20, 30}, want: []stream.Pair[int, int]{{First: 1, Second: 10}, {First: 2, Second: 20}}, leftConsumed: 2, rightConsumed: 2, rightStarted: true},
		{name: "right shorter", left: []int{1, 2, 3}, right: []int{10, 20}, want: []stream.Pair[int, int]{{First: 1, Second: 10}, {First: 2, Second: 20}}, leftConsumed: 3, rightConsumed: 2, rightStarted: true},
		{name: "equal", left: []int{1, 2}, right: []int{10, 20}, want: []stream.Pair[int, int]{{First: 1, Second: 10}, {First: 2, Second: 20}}, leftConsumed: 2, rightConsumed: 2, rightStarted: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			leftProbe := new(sequenceProbe)
			rightProbe := new(sequenceProbe)
			got := stream.Zip(
				stream.FromSeq(instrumentedSeq(test.left, leftProbe)),
				stream.FromSeq(instrumentedSeq(test.right, rightProbe)),
			).Collect()
			if !slices.Equal(got, test.want) {
				t.Fatalf("Zip = %v, want %v", got, test.want)
			}
			if leftProbe.consumed.Load() != test.leftConsumed || rightProbe.consumed.Load() != test.rightConsumed {
				t.Fatalf("consumed left=%d right=%d, want left=%d right=%d", leftProbe.consumed.Load(), rightProbe.consumed.Load(), test.leftConsumed, test.rightConsumed)
			}
			if (rightProbe.starts.Load() != 0) != test.rightStarted {
				t.Fatalf("right starts = %d, want started %v", rightProbe.starts.Load(), test.rightStarted)
			}
			if leftProbe.cleanups.Load() != 1 {
				t.Fatalf("left cleanup = %d, want 1", leftProbe.cleanups.Load())
			}
			wantRightCleanup := int64(0)
			if test.rightStarted {
				wantRightCleanup = 1
			}
			if rightProbe.cleanups.Load() != wantRightCleanup {
				t.Fatalf("right cleanup = %d, want %d", rightProbe.cleanups.Load(), wantRightCleanup)
			}
		})
	}
}

func TestZipDownstreamTermination(t *testing.T) {
	leftProbe := new(sequenceProbe)
	rightProbe := new(sequenceProbe)
	zipped := stream.Zip(
		stream.FromSeq(instrumentedSeq([]int{1, 2, 3}, leftProbe)),
		stream.FromSeq(instrumentedSeq([]int{10, 20, 30}, rightProbe)),
	)
	if got := zipped.First().Must(); got != (stream.Pair[int, int]{First: 1, Second: 10}) {
		t.Fatalf("Zip First = %v", got)
	}
	if leftProbe.consumed.Load() != 1 || rightProbe.consumed.Load() != 1 {
		t.Fatalf("early stop consumed left=%d right=%d", leftProbe.consumed.Load(), rightProbe.consumed.Load())
	}
	if leftProbe.cleanups.Load() != 1 || rightProbe.cleanups.Load() != 1 {
		t.Fatalf("early stop cleanup left=%d right=%d", leftProbe.cleanups.Load(), rightProbe.cleanups.Load())
	}
}

func TestZipCleanupOnPanics(t *testing.T) {
	t.Run("left panic", func(t *testing.T) {
		var leftCleaned, rightCleaned atomic.Bool
		left := stream.FromSeq(func(yield func(int) bool) {
			defer leftCleaned.Store(true)
			if !yield(1) {
				return
			}
			panic("left")
		})
		right := stream.FromSeq(func(yield func(int) bool) {
			defer rightCleaned.Store(true)
			for value := 10; ; value++ {
				if !yield(value) {
					return
				}
			}
		})
		requirePanics(t, "left source", func() { stream.Zip(left, right).Collect() })
		if !leftCleaned.Load() || !rightCleaned.Load() {
			t.Fatalf("cleanup left=%v right=%v", leftCleaned.Load(), rightCleaned.Load())
		}
	})

	t.Run("right panic", func(t *testing.T) {
		var leftCleaned, rightCleaned atomic.Bool
		left := stream.FromSeq(func(yield func(int) bool) {
			defer leftCleaned.Store(true)
			for value := 1; ; value++ {
				if !yield(value) {
					return
				}
			}
		})
		right := stream.FromSeq(func(yield func(int) bool) {
			defer rightCleaned.Store(true)
			if !yield(10) {
				return
			}
			panic("right")
		})
		requirePanics(t, "right source", func() { stream.Zip(left, right).Collect() })
		if !leftCleaned.Load() || !rightCleaned.Load() {
			t.Fatalf("cleanup left=%v right=%v", leftCleaned.Load(), rightCleaned.Load())
		}
	})

	t.Run("downstream panic", func(t *testing.T) {
		leftProbe := new(sequenceProbe)
		rightProbe := new(sequenceProbe)
		zipped := stream.Zip(
			stream.FromSeq(instrumentedSeq([]int{1, 2}, leftProbe)),
			stream.FromSeq(instrumentedSeq([]int{10, 20}, rightProbe)),
		)
		requirePanics(t, "downstream", func() {
			zipped.Seq()(func(stream.Pair[int, int]) bool { panic("downstream") })
		})
		if leftProbe.cleanups.Load() != 1 || rightProbe.cleanups.Load() != 1 {
			t.Fatalf("cleanup left=%d right=%d", leftProbe.cleanups.Load(), rightProbe.cleanups.Load())
		}
	})
}

func TestZipNextAndStopAreNotConcurrent(t *testing.T) {
	var inCallback atomic.Int32
	var concurrent atomic.Bool
	right := stream.FromSeq(func(yield func(int) bool) {
		defer func() {
			if inCallback.Load() != 0 {
				concurrent.Store(true)
			}
		}()
		for value := 0; ; value++ {
			if !inCallback.CompareAndSwap(0, 1) {
				concurrent.Store(true)
			}
			accepted := yield(value)
			inCallback.Store(0)
			if !accepted {
				return
			}
		}
	})
	stream.Zip(stream.Range(0, 10), right).Take(3).Collect()
	if concurrent.Load() {
		t.Fatal("right iterator next and stop overlapped")
	}
}
