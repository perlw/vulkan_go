package main

import (
	"fmt"
	"runtime"

	"github.com/perlw/harle"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	//"github.com/go-gl/mathgl/mgl32"
	//"github.com/rthornton128/goncurses"
)

func init() {
	runtime.LockOSThread()
	runtime.GOMAXPROCS(2)
}

const width = 1280
const height = 720

func main() {
	fmt.Println("main")
	harle.Foo()

	// +Setup GLFW
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	window, err := glfw.CreateWindow(width, height, "Testing", nil, nil)
	if err != nil {
		panic(err)
	}

	window.SetKeyCallback(func(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
		if key == glfw.KeyEscape {
			w.SetShouldClose(true)
		}
	})
	window.MakeContextCurrent()
	glfw.SwapInterval(0)
	// -Setup GLFW

	// +Setup GL
	if err := gl.Init(); err != nil {
		panic(err)
	}

	var major int32
	var minor int32
	gl.GetIntegerv(gl.MAJOR_VERSION, &major)
	gl.GetIntegerv(gl.MINOR_VERSION, &minor)
	fmt.Printf("GL %d.%d\n", major, minor)

	gl.Enable(gl.CULL_FACE)
	gl.Enable(gl.DEPTH_TEST)
	gl.ClearDepth(1)
	gl.DepthFunc(gl.LESS)
	gl.Viewport(0, 0, width, height)
	gl.ClearColor(0.5, 0.5, 1.0, 1.0)
	// -Setup GL

	var tick float64 = 0.0
	var frames uint32 = 0
	for !window.ShouldClose() {
		time := glfw.GetTime()
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		window.SwapBuffers()
		glfw.PollEvents()

		frames++
		if time-tick >= 1.0 {
			fmt.Printf("FPS: %d\n", frames)
			frames = 0
			tick = time
		}
	}
}
