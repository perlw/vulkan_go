package main

import (
	"fmt"
	"runtime"
)

func init() {
	runtime.LockOSThread()
	runtime.GOMAXPROCS(2)
}

func main() {
	fmt.Println("foobar")
}
