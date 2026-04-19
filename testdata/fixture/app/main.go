package main

import (
	"fmt"

	"fixture/greet"
)

func main() {
	g := greet.NewGreeter("world")
	fmt.Println(g.Hello())
	run()
}

func run() {
	fmt.Println(util())
	doubled := greet.Map([]int{1, 2, 3}, double)
	fmt.Println(doubled)
}
