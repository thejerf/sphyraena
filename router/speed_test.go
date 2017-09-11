package router

// Reference for features was also made to
// https://github.com/ant0ine/go-urlrouter/blob/master/router_benchmark_test.go
// which has the advantage of being a semi-real attempt at specifying
// routing.

import "testing"

// assigning to this can get around the "not used" error
var dump int

// It would be most convenient for our design if calling through a closure
// was comparable to calling a method.
func BenchmarkClosureCallSpeed(b *testing.B) {
	var j int

	incr := func() {
		j++
	}

	for i := 0; i < b.N; i++ {
		incr()
	}

	dump = j
}

type Node struct {
	j int
}

func (n *Node) incr() {
	n.j++
}

func BenchmarkProbablyInlinedMethodCall(b *testing.B) {
	n := &Node{}

	for i := 0; i < b.N; i++ {
		n.incr()
	}
}

type Incr interface {
	Incr()
}

type IncrClosure interface {
	IncrClosure() func()
}

type Incrementer struct {
	i int
}

func (i *Incrementer) Incr() {
	i.i++
}

func (i *Incrementer) IncrClosure() func() {
	return func() {
		i.i++
	}
}

func BenchmarkCallingThroughInterface(b *testing.B) {
	var incr Incr
	incr = &Incrementer{0}

	for i := 0; i < b.N; i++ {
		incr.Incr()
	}
}

func BenchmarkCallingThroughClosure(b *testing.B) {
	incr := &Incrementer{0}
	c := incr.IncrClosure()

	for i := 0; i < b.N; i++ {
		c()
	}
}
