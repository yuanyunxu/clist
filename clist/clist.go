// Package clist
package clist

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type IntList struct {
	head   *intNode
	length int64
}

type intNode struct {
	value  int
	next   *intNode
	mu     sync.Mutex
	marked int32
}

func newIntNode(value int) *intNode {
	return &intNode{value: value}
}

func NewInt() *IntList {
	return &IntList{head: newIntNode(-1)}
}

func (l *IntList) Insert(value int) bool {
	var a *intNode
	var b *intNode

	for {
		// Step 1, 寻找a b
		a = l.head
		b = a.atomicNext()

		for b != nil && b.value < value {
			a = b
			b = b.atomicNext()
		}
		// Check if the node is exist.
		if b != nil && b.value == value {
			return false
		}

		// Step 2, 锁定a, 如果b变更了，那么之前找到a和b不再准确，需要重新寻找
		a.mu.Lock()
		if a.next != b {
			a.mu.Unlock()
			continue
		}
		break
	}

	// Step 3， 创建插入节点
	x := newIntNode(value)

	// Step 4, 链表中插入节点
	x.next = b
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&a.next)), unsafe.Pointer(x))
	atomic.AddInt64(&l.length, 1)

	// Step 5, 解锁a节点
	defer a.mu.Unlock()

	return true
}

func (l *IntList) Delete(value int) bool {
	var a *intNode
	var b *intNode

	for {
		// Step 1, 找到a，b，不存在直接返回
		a = l.head
		b = a.atomicNext()

		for b != nil && b.value < value {
			a = b
			b = b.atomicNext()
		}
		// Check if b is not exists
		if b == nil || b.value != value {
			return false
		}

		// Step 2, 锁定并检查b, 如果链表状态已经改变，那么重新查找a b
		b.mu.Lock()
		if atomic.LoadInt32(&b.marked) > 0 {
			b.mu.Unlock()
			continue
		}
		// Step 3，锁定并检查a，如果a变更了，说明此时a不再准确，重新查找
		a.mu.Lock()
		if atomic.LoadInt32(&a.marked) > 0 || a.next != b {
			a.mu.Unlock()
			b.mu.Unlock()
			continue
		}
		break
	}

	// Step 4, 删除b
	atomic.AddInt32(&b.marked, 1)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&a.next)), unsafe.Pointer(b.next))
	atomic.AddInt64(&l.length, -1)

	// Step 5， 解锁a b
	defer func() {
		a.mu.Unlock()
		b.mu.Unlock()
	}()

	return true
}

func (l *IntList) Contains(value int) bool {
	x := l.head.atomicNext()
	for x != nil && x.value < value {
		x = x.atomicNext()
	}

	if x == nil {
		return false
	}

	return x.value == value && atomic.LoadInt32(&x.marked) == 0
}

func (l *IntList) Range(f func(value int) bool) {
	x := l.head.atomicNext()
	for x != nil {
		if !f(x.value) {
			break
		}
		x = x.atomicNext()
	}
}

func (l *IntList) Len() int {
	return int(atomic.LoadInt64(&l.length))
}

func (n *intNode) atomicNext() *intNode {
	return (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&n.next))))
}
