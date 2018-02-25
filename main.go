package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/perlw/harle"

	"github.com/nsf/termbox-go"
)

func init() {
	runtime.LockOSThread()
	runtime.GOMAXPROCS(2)
}

func main() {
	fmt.Println("main")
	harle.Foo()

	fmt.Printf("Termbox isInit? %t\n", termbox.IsInit)

	if err := termbox.Init(); err != nil {
		panic(err)
	}
	defer termbox.Close()

	fmt.Printf("Termbox isInit? %t\n", termbox.IsInit)

	if termbox.SetOutputMode(termbox.Output256) != termbox.Output256 {
		fmt.Println("Could not set 256")
	}
	fmt.Println(termbox.SetOutputMode(termbox.Output256))
	fmt.Println(termbox.Size())

	count := 0
	toggle := false
	tick := time.Tick(time.Second)
	for range tick {
		count++
		if count > 10 {
			break
		}

		if toggle {
			termbox.SetCell(0, 0, '@', termbox.ColorBlack, termbox.ColorBlack)
		} else {
			termbox.SetCell(0, 0, '@', termbox.ColorWhite, termbox.ColorBlack)
		}
		toggle = !toggle

		termbox.Flush()
	}
}
