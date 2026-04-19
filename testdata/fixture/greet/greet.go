package greet

import "fmt"

type Greeter struct {
	Name string
}

func NewGreeter(name string) *Greeter {
	return &Greeter{Name: name}
}

func (g *Greeter) Hello() string {
	return helper(g.Name)
}

func helper(name string) string {
	return fmt.Sprintf("hello, %s", name)
}

func Map[T any, U any](xs []T, f func(T) U) []U {
	out := make([]U, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}
