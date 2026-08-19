package predicate_test

import (
	"testing"
	"unsafe"

	"github.com/imbrooklyn/shuttle/predicate"
)

func FuzzDoubleNegation(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 255} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, bits uint8) {
		result := bits&1 != 0
		base := predicate.Always[int](result)
		if got := base.Not().Not()(int(bits)); got != result {
			t.Fatalf("double negation = %v, want %v", got, result)
		}
	})
}

func FuzzBooleanLaws(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 3, 5, 7, 255} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, bits uint8) {
		a := bits&1 != 0
		b := bits&2 != 0
		c := bits&4 != 0
		pa := predicate.Always[struct{}](a)
		pb := predicate.Always[struct{}](b)
		pc := predicate.Always[struct{}](c)
		value := struct{}{}

		if got, want := pa.And(pb, pc)(value), a && b && c; got != want {
			t.Fatalf("And truth table = %v, want %v", got, want)
		}
		if got, want := pa.Or(pb, pc)(value), a || b || c; got != want {
			t.Fatalf("Or truth table = %v, want %v", got, want)
		}
		if got, want := pa.And(pb).Not()(value), pa.Not().Or(pb.Not())(value); got != want {
			t.Fatalf("And De Morgan = %v, want %v", got, want)
		}
		if got, want := pa.Or(pb).Not()(value), pa.Not().And(pb.Not())(value); got != want {
			t.Fatalf("Or De Morgan = %v, want %v", got, want)
		}
	})
}

func FuzzIsNotNilNegation(f *testing.F) {
	f.Add(uint8(0), []byte(nil))
	f.Add(uint8(1), []byte("value"))
	f.Add(uint8(11), []byte{})
	f.Fuzz(func(t *testing.T, selector uint8, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var value any
		switch selector % 12 {
		case 0:
			value = nil
		case 1:
			value = (*int)(nil)
		case 2:
			value = []byte(nil)
		case 3:
			copyOfData := make([]byte, len(data))
			copy(copyOfData, data)
			value = copyOfData
		case 4:
			value = map[string]int(nil)
		case 5:
			value = map[string]int{"length": len(data)}
		case 6:
			value = (func())(nil)
		case 7:
			value = func() {}
		case 8:
			value = (chan int)(nil)
		case 9:
			value = unsafe.Pointer(nil)
		case 10:
			marker := byte(0)
			value = unsafe.Pointer(&marker)
		default:
			value = len(data)
		}

		if got, want := predicate.IsNotNil(value), !predicate.IsNil(value); got != want {
			t.Fatalf("IsNotNil(%T) = %v, want %v", value, got, want)
		}
	})
}

func FuzzCompositionMatchesReference(f *testing.F) {
	f.Add(0, 0, 0)
	f.Add(2, 3, 2)
	f.Add(-1, 5, -1)
	f.Add(100, -20, 7)
	f.Fuzz(func(t *testing.T, value, upper, exact int) {
		nonNegative := predicate.Func[int](func(current int) bool { return current >= 0 })
		even := predicate.Func[int](func(current int) bool { return current%2 == 0 })
		belowUpper := predicate.Func[int](func(current int) bool { return current < upper })
		equalsExact := predicate.Equal(exact)

		composed := nonNegative.And(even, belowUpper).Or(equalsExact).Not()
		want := !((value >= 0 && value%2 == 0 && value < upper) || value == exact)
		if got := composed(value); got != want {
			t.Fatalf("composition(%d, %d, %d) = %v, want %v", value, upper, exact, got, want)
		}
	})
}
