package hw04lrucache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func forward(t *testing.T, l List) []int {
	t.Helper()

	elems := make([]int, 0, l.Len())
	for i := l.Front(); i != nil; i = i.Next {
		v, ok := i.Value.(int)
		require.True(t, ok)
		elems = append(elems, v)
	}
	return elems
}

func backward(t *testing.T, l List) []int {
	t.Helper()

	elems := make([]int, 0, l.Len())
	for i := l.Back(); i != nil; i = i.Prev {
		v, ok := i.Value.(int)
		require.True(t, ok)
		elems = append(elems, v)
	}
	return elems
}

func requireList(t *testing.T, l List, want []int) {
	t.Helper()

	require.Equal(t, len(want), l.Len())
	require.Equal(t, want, forward(t, l))

	reversed := make([]int, len(want))
	for i, v := range want {
		reversed[len(want)-1-i] = v
	}
	require.Equal(t, reversed, backward(t, l))

	if len(want) == 0 {
		require.Nil(t, l.Front())
		require.Nil(t, l.Back())
		return
	}

	require.Equal(t, want[0], l.Front().Value)
	require.Equal(t, want[len(want)-1], l.Back().Value)
	require.Nil(t, l.Front().Prev, "у головы не должно быть Prev")
	require.Nil(t, l.Back().Next, "у хвоста не должно быть Next")
}

func TestList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		l := NewList()

		require.Equal(t, 0, l.Len())
		require.Nil(t, l.Front())
		require.Nil(t, l.Back())
	})

	t.Run("complex", func(t *testing.T) {
		l := NewList()

		l.PushFront(10) // [10]
		l.PushBack(20)  // [10, 20]
		l.PushBack(30)  // [10, 20, 30]
		require.Equal(t, 3, l.Len())

		middle := l.Front().Next // 20
		l.Remove(middle)         // [10, 30]
		require.Equal(t, 2, l.Len())

		for i, v := range [...]int{40, 50, 60, 70, 80} {
			if i%2 == 0 {
				l.PushFront(v)
			} else {
				l.PushBack(v)
			}
		} // [80, 60, 40, 10, 30, 50, 70]

		require.Equal(t, 7, l.Len())
		require.Equal(t, 80, l.Front().Value)
		require.Equal(t, 70, l.Back().Value)

		l.MoveToFront(l.Front()) // [80, 60, 40, 10, 30, 50, 70]
		l.MoveToFront(l.Back())  // [70, 80, 60, 40, 10, 30, 50]

		elems := make([]int, 0, l.Len())
		for i := l.Front(); i != nil; i = i.Next {
			elems = append(elems, i.Value.(int)) //nolint:forcetypeassert
		}
		require.Equal(t, []int{70, 80, 60, 40, 10, 30, 50}, elems)
	})

	t.Run("single element", func(t *testing.T) {
		l := NewList()

		item := l.PushFront(1)
		require.Same(t, item, l.Front())
		require.Same(t, item, l.Back())
		requireList(t, l, []int{1})

		l.MoveToFront(item)
		requireList(t, l, []int{1})

		l.Remove(item)
		requireList(t, l, []int{})

		l.PushBack(2)
		requireList(t, l, []int{2})
	})

	t.Run("push order", func(t *testing.T) {
		l := NewList()

		l.PushBack(2)
		l.PushBack(3)
		l.PushFront(1)
		l.PushFront(0)
		requireList(t, l, []int{0, 1, 2, 3})
	})

	t.Run("remove head and tail", func(t *testing.T) {
		l := NewList()
		for _, v := range []int{1, 2, 3, 4} {
			l.PushBack(v)
		}

		l.Remove(l.Front())
		requireList(t, l, []int{2, 3, 4})

		l.Remove(l.Back())
		requireList(t, l, []int{2, 3})

		l.Remove(l.Front())
		l.Remove(l.Front())
		requireList(t, l, []int{})
	})

	t.Run("removed item is detached", func(t *testing.T) {
		l := NewList()
		l.PushBack(1)
		middle := l.PushBack(2)
		l.PushBack(3)

		l.Remove(middle)

		require.Nil(t, middle.Prev, "удалённый элемент не должен ссылаться на список")
		require.Nil(t, middle.Next)
		require.Equal(t, 2, middle.Value, "значение удалённого элемента сохраняется")
		requireList(t, l, []int{1, 3})
	})

	t.Run("move to front keeps identity", func(t *testing.T) {
		l := NewList()
		first := l.PushBack(1)
		middle := l.PushBack(2)
		last := l.PushBack(3)

		l.MoveToFront(middle)
		requireList(t, l, []int{2, 1, 3})
		require.Same(t, middle, l.Front(), "узел должен переиспользоваться, а не пересоздаваться")

		l.MoveToFront(last)
		requireList(t, l, []int{3, 2, 1})
		require.Same(t, last, l.Front())
		require.Same(t, first, l.Back())

		l.MoveToFront(last)
		requireList(t, l, []int{3, 2, 1})
	})

	t.Run("move every element to front", func(t *testing.T) {
		want := []int{1, 2, 3, 4, 5}

		l := NewList()
		for _, v := range want {
			l.PushBack(v)
		}

		for range want {
			l.MoveToFront(l.Back())
		}

		requireList(t, l, want)
	})

	t.Run("lists are independent", func(t *testing.T) {
		a := NewList()
		b := NewList()

		a.PushBack(1)
		b.PushBack(2)

		requireList(t, a, []int{1})
		requireList(t, b, []int{2})
	})

	t.Run("mixed value types", func(t *testing.T) {
		l := NewList()

		l.PushBack("str")
		l.PushBack(nil)
		l.PushFront(struct{ A int }{A: 1})

		require.Equal(t, 3, l.Len())
		require.Equal(t, struct{ A int }{A: 1}, l.Front().Value)
		require.Nil(t, l.Back().Value)
	})
}
