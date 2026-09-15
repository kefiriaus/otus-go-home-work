package hw04lrucache

type List interface {
	Len() int
	Front() *ListItem
	Back() *ListItem
	PushFront(v interface{}) *ListItem
	PushBack(v interface{}) *ListItem
	Remove(i *ListItem)
	MoveToFront(i *ListItem)
}

type ListItem struct {
	Value interface{}
	Next  *ListItem
	Prev  *ListItem
}

type list struct {
	front *ListItem
	back  *ListItem
	len   int
}

func NewList() List {
	return new(list)
}

func (l *list) Len() int {
	return l.len
}

func (l *list) Front() *ListItem {
	return l.front
}

func (l *list) Back() *ListItem {
	return l.back
}

func (l *list) PushFront(v interface{}) *ListItem {
	i := &ListItem{Value: v}
	l.linkFront(i)
	return i
}

func (l *list) PushBack(v interface{}) *ListItem {
	i := &ListItem{Value: v, Prev: l.back}
	if l.back != nil {
		l.back.Next = i
	} else {
		l.front = i
	}
	l.back = i
	l.len++
	return i
}

func (l *list) Remove(i *ListItem) {
	if i == nil {
		return
	}
	l.unlink(i)
	i.Prev, i.Next = nil, nil
}

func (l *list) MoveToFront(i *ListItem) {
	if i == nil || i == l.front {
		return
	}
	l.unlink(i)
	l.linkFront(i)
}

func (l *list) linkFront(i *ListItem) {
	i.Prev = nil
	i.Next = l.front
	if l.front != nil {
		l.front.Prev = i
	} else {
		l.back = i
	}
	l.front = i
	l.len++
}

func (l *list) unlink(i *ListItem) {
	if i.Prev != nil {
		i.Prev.Next = i.Next
	} else {
		l.front = i.Next
	}
	if i.Next != nil {
		i.Next.Prev = i.Prev
	} else {
		l.back = i.Prev
	}
	l.len--
}
