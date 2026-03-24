package wazevoapi

// Queue is the resettable queue where the underlying slice is reused.
type Queue[T any] struct {
	index int
	Data  []T
}

func (q *Queue[T]) Enqueue(v T) {
	q.Data = append(q.Data, v)
}

func (q *Queue[T]) Dequeue() (ret T) {
	ret = q.Data[q.index]
	q.index++
	return
}

func (q *Queue[T]) Empty() bool {
	return q.index >= len(q.Data)
}

func (q *Queue[T]) Reset() {
	q.index = 0
	q.Data = q.Data[:0]
}
