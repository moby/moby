package mapset

import (
	"math/rand"
	"testing"
)

func nrand(n int) []int {
	i := make([]int, n)
	for ind := range i {
		i[ind] = rand.Int()
	}
	return i
}

func toInterfaces(i []int) []interface{} {
	ifs := make([]interface{}, len(i))
	for ind, v := range i {
		ifs[ind] = v
	}
	return ifs
}

func benchAdd(b *testing.B, s Set) {
	nums := nrand(b.N)
	b.ResetTimer()
	for _, v := range nums {
		s.Add(v)
	}
}

func BenchmarkAddSafe(b *testing.B) {
	benchAdd(b, NewSet())
}

func BenchmarkAddUnsafe(b *testing.B) {
	benchAdd(b, NewThreadUnsafeSet())
}

func benchRemove(b *testing.B, s Set) {
	nums := nrand(b.N)
	for _, v := range nums {
		s.Add(v)
	}

	b.ResetTimer()
	for _, v := range nums {
		s.Remove(v)
	}
}

func BenchmarkRemoveSafe(b *testing.B) {
	benchRemove(b, NewSet())
}

func BenchmarkRemoveUnsafe(b *testing.B) {
	benchRemove(b, NewThreadUnsafeSet())
}

func benchCardinality(b *testing.B, s Set) {
	for i := 0; i < b.N; i++ {
		s.Cardinality()
	}
}

func BenchmarkCardinalitySafe(b *testing.B) {
	benchCardinality(b, NewSet())
}

func BenchmarkCardinalityUnsafe(b *testing.B) {
	benchCardinality(b, NewThreadUnsafeSet())
}

func benchClear(b *testing.B, s Set) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Clear()
	}
}

func BenchmarkClearSafe(b *testing.B) {
	benchClear(b, NewSet())
}

func BenchmarkClearUnsafe(b *testing.B) {
	benchClear(b, NewThreadUnsafeSet())
}

func benchClone(b *testing.B, n int, s Set) {
	nums := toInterfaces(nrand(n))
	for _, v := range nums {
		s.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Clone()
	}
}

func BenchmarkClone1Safe(b *testing.B) {
	benchClone(b, 1, NewSet())
}

func BenchmarkClone1Unsafe(b *testing.B) {
	benchClone(b, 1, NewThreadUnsafeSet())
}

func BenchmarkClone10Safe(b *testing.B) {
	benchClone(b, 10, NewSet())
}

func BenchmarkClone10Unsafe(b *testing.B) {
	benchClone(b, 10, NewThreadUnsafeSet())
}

func BenchmarkClone100Safe(b *testing.B) {
	benchClone(b, 100, NewSet())
}

func BenchmarkClone100Unsafe(b *testing.B) {
	benchClone(b, 100, NewThreadUnsafeSet())
}

func benchContains(b *testing.B, n int, s Set) {
	nums := toInterfaces(nrand(n))
	for _, v := range nums {
		s.Add(v)
	}

	nums[n-1] = -1 // Definitely not in s

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Contains(nums...)
	}
}

func BenchmarkContains1Safe(b *testing.B) {
	benchContains(b, 1, NewSet())
}

func BenchmarkContains1Unsafe(b *testing.B) {
	benchContains(b, 1, NewThreadUnsafeSet())
}

func BenchmarkContains10Safe(b *testing.B) {
	benchContains(b, 10, NewSet())
}

func BenchmarkContains10Unsafe(b *testing.B) {
	benchContains(b, 10, NewThreadUnsafeSet())
}

func BenchmarkContains100Safe(b *testing.B) {
	benchContains(b, 100, NewSet())
}

func BenchmarkContains100Unsafe(b *testing.B) {
	benchContains(b, 100, NewThreadUnsafeSet())
}

func benchEqual(b *testing.B, n int, s, t Set) {
	nums := nrand(n)
	for _, v := range nums {
		s.Add(v)
		t.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Equal(t)
	}
}

func BenchmarkEqual1Safe(b *testing.B) {
	benchEqual(b, 1, NewSet(), NewSet())
}

func BenchmarkEqual1Unsafe(b *testing.B) {
	benchEqual(b, 1, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkEqual10Safe(b *testing.B) {
	benchEqual(b, 10, NewSet(), NewSet())
}

func BenchmarkEqual10Unsafe(b *testing.B) {
	benchEqual(b, 10, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkEqual100Safe(b *testing.B) {
	benchEqual(b, 100, NewSet(), NewSet())
}

func BenchmarkEqual100Unsafe(b *testing.B) {
	benchEqual(b, 100, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func benchDifference(b *testing.B, n int, s, t Set) {
	nums := nrand(n)
	for _, v := range nums {
		s.Add(v)
	}
	for _, v := range nums[:n/2] {
		t.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Difference(t)
	}
}

func benchIsSubset(b *testing.B, n int, s, t Set) {
	nums := nrand(n)
	for _, v := range nums {
		s.Add(v)
		t.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.IsSubset(t)
	}
}

func BenchmarkIsSubset1Safe(b *testing.B) {
	benchIsSubset(b, 1, NewSet(), NewSet())
}

func BenchmarkIsSubset1Unsafe(b *testing.B) {
	benchIsSubset(b, 1, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkIsSubset10Safe(b *testing.B) {
	benchIsSubset(b, 10, NewSet(), NewSet())
}

func BenchmarkIsSubset10Unsafe(b *testing.B) {
	benchIsSubset(b, 10, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkIsSubset100Safe(b *testing.B) {
	benchIsSubset(b, 100, NewSet(), NewSet())
}

func BenchmarkIsSubset100Unsafe(b *testing.B) {
	benchIsSubset(b, 100, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func benchIsSuperset(b *testing.B, n int, s, t Set) {
	nums := nrand(n)
	for _, v := range nums {
		s.Add(v)
		t.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.IsSuperset(t)
	}
}

func BenchmarkIsSuperset1Safe(b *testing.B) {
	benchIsSuperset(b, 1, NewSet(), NewSet())
}

func BenchmarkIsSuperset1Unsafe(b *testing.B) {
	benchIsSuperset(b, 1, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkIsSuperset10Safe(b *testing.B) {
	benchIsSuperset(b, 10, NewSet(), NewSet())
}

func BenchmarkIsSuperset10Unsafe(b *testing.B) {
	benchIsSuperset(b, 10, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkIsSuperset100Safe(b *testing.B) {
	benchIsSuperset(b, 100, NewSet(), NewSet())
}

func BenchmarkIsSuperset100Unsafe(b *testing.B) {
	benchIsSuperset(b, 100, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkDifference1Safe(b *testing.B) {
	benchDifference(b, 1, NewSet(), NewSet())
}

func BenchmarkDifference1Unsafe(b *testing.B) {
	benchDifference(b, 1, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkDifference10Safe(b *testing.B) {
	benchDifference(b, 10, NewSet(), NewSet())
}

func BenchmarkDifference10Unsafe(b *testing.B) {
	benchDifference(b, 10, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkDifference100Safe(b *testing.B) {
	benchDifference(b, 100, NewSet(), NewSet())
}

func BenchmarkDifference100Unsafe(b *testing.B) {
	benchDifference(b, 100, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func benchIntersect(b *testing.B, n int, s, t Set) {
	nums := nrand(int(float64(n) * float64(1.5)))
	for _, v := range nums[:n] {
		s.Add(v)
	}
	for _, v := range nums[n/2:] {
		t.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Intersect(t)
	}
}

func BenchmarkIntersect1Safe(b *testing.B) {
	benchIntersect(b, 1, NewSet(), NewSet())
}

func BenchmarkIntersect1Unsafe(b *testing.B) {
	benchIntersect(b, 1, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkIntersect10Safe(b *testing.B) {
	benchIntersect(b, 10, NewSet(), NewSet())
}

func BenchmarkIntersect10Unsafe(b *testing.B) {
	benchIntersect(b, 10, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkIntersect100Safe(b *testing.B) {
	benchIntersect(b, 100, NewSet(), NewSet())
}

func BenchmarkIntersect100Unsafe(b *testing.B) {
	benchIntersect(b, 100, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func benchSymmetricDifference(b *testing.B, n int, s, t Set) {
	nums := nrand(int(float64(n) * float64(1.5)))
	for _, v := range nums[:n] {
		s.Add(v)
	}
	for _, v := range nums[n/2:] {
		t.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SymmetricDifference(t)
	}
}

func BenchmarkSymmetricDifference1Safe(b *testing.B) {
	benchSymmetricDifference(b, 1, NewSet(), NewSet())
}

func BenchmarkSymmetricDifference1Unsafe(b *testing.B) {
	benchSymmetricDifference(b, 1, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkSymmetricDifference10Safe(b *testing.B) {
	benchSymmetricDifference(b, 10, NewSet(), NewSet())
}

func BenchmarkSymmetricDifference10Unsafe(b *testing.B) {
	benchSymmetricDifference(b, 10, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkSymmetricDifference100Safe(b *testing.B) {
	benchSymmetricDifference(b, 100, NewSet(), NewSet())
}

func BenchmarkSymmetricDifference100Unsafe(b *testing.B) {
	benchSymmetricDifference(b, 100, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func benchUnion(b *testing.B, n int, s, t Set) {
	nums := nrand(n)
	for _, v := range nums[:n/2] {
		s.Add(v)
	}
	for _, v := range nums[n/2:] {
		t.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Union(t)
	}
}

func BenchmarkUnion1Safe(b *testing.B) {
	benchUnion(b, 1, NewSet(), NewSet())
}

func BenchmarkUnion1Unsafe(b *testing.B) {
	benchUnion(b, 1, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkUnion10Safe(b *testing.B) {
	benchUnion(b, 10, NewSet(), NewSet())
}

func BenchmarkUnion10Unsafe(b *testing.B) {
	benchUnion(b, 10, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func BenchmarkUnion100Safe(b *testing.B) {
	benchUnion(b, 100, NewSet(), NewSet())
}

func BenchmarkUnion100Unsafe(b *testing.B) {
	benchUnion(b, 100, NewThreadUnsafeSet(), NewThreadUnsafeSet())
}

func benchIter(b *testing.B, n int, s Set) {
	nums := nrand(n)
	for _, v := range nums {
		s.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := s.Iter()
		for _ = range c {

		}
	}
}

func BenchmarkIter1Safe(b *testing.B) {
	benchIter(b, 1, NewSet())
}

func BenchmarkIter1Unsafe(b *testing.B) {
	benchIter(b, 1, NewThreadUnsafeSet())
}

func BenchmarkIter10Safe(b *testing.B) {
	benchIter(b, 10, NewSet())
}

func BenchmarkIter10Unsafe(b *testing.B) {
	benchIter(b, 10, NewThreadUnsafeSet())
}

func BenchmarkIter100Safe(b *testing.B) {
	benchIter(b, 100, NewSet())
}

func BenchmarkIter100Unsafe(b *testing.B) {
	benchIter(b, 100, NewThreadUnsafeSet())
}

func benchString(b *testing.B, n int, s Set) {
	nums := nrand(n)
	for _, v := range nums {
		s.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.String()
	}
}

func BenchmarkString1Safe(b *testing.B) {
	benchString(b, 1, NewSet())
}

func BenchmarkString1Unsafe(b *testing.B) {
	benchString(b, 1, NewThreadUnsafeSet())
}

func BenchmarkString10Safe(b *testing.B) {
	benchString(b, 10, NewSet())
}

func BenchmarkString10Unsafe(b *testing.B) {
	benchString(b, 10, NewThreadUnsafeSet())
}

func BenchmarkString100Safe(b *testing.B) {
	benchString(b, 100, NewSet())
}

func BenchmarkString100Unsafe(b *testing.B) {
	benchString(b, 100, NewThreadUnsafeSet())
}

func benchToSlice(b *testing.B, s Set) {
	nums := nrand(b.N)
	for _, v := range nums {
		s.Add(v)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ToSlice()
	}
}

func BenchmarkToSliceSafe(b *testing.B) {
	benchToSlice(b, NewSet())
}

func BenchmarkToSliceUnsafe(b *testing.B) {
	benchToSlice(b, NewThreadUnsafeSet())
}
