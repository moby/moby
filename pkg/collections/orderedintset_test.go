package collections

import (
	"math/rand"
	"testing"
)

func BenchmarkPush(b *testing.B) {
	var testSet []int
	for i := 0; i < 1000; i++ {
		testSet = append(testSet, rand.Int())
	}
	s := NewOrderedIntSet()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, elem := range testSet {
			s.Push(elem)
		}
	}
}

func BenchmarkPop(b *testing.B) {
	var testSet []int
	for i := 0; i < 1000; i++ {
		testSet = append(testSet, rand.Int())
	}
	s := NewOrderedIntSet()
	for _, elem := range testSet {
		s.Push(elem)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			s.Pop()
		}
	}
}

func BenchmarkExist(b *testing.B) {
	var testSet []int
	for i := 0; i < 1000; i++ {
		testSet = append(testSet, rand.Intn(2000))
	}
	s := NewOrderedIntSet()
	for _, elem := range testSet {
		s.Push(elem)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			s.Exists(j)
		}
	}
}

func BenchmarkRemove(b *testing.B) {
	var testSet []int
	for i := 0; i < 1000; i++ {
		testSet = append(testSet, rand.Intn(2000))
	}
	s := NewOrderedIntSet()
	for _, elem := range testSet {
		s.Push(elem)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			s.Remove(j)
		}
	}
}
