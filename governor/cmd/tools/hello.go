package main

import "fmt"

func SayHello(name string) string {
	if name == "" {
		return "Hello, World!"
	}
	return "Hello, " + name + "!"
}

func main() {
	fmt.Println(SayHello(""))
	fmt.Println(SayHello("VibePilot!"))
}
