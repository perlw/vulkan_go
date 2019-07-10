package pompeii

import (
	"github.com/pkg/errors"
	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

type Surface interface {
	Handle() vk.Surface
	Destroy()
}

type windowSurfaceVk struct {
	surface vk.Surface
}

type WindowSurface struct {
	instance *Instance

	vk *windowSurfaceVk
}

func NewWindowSurface(instance *Instance, window *glfw.Window) (*WindowSurface, error) {
	w := WindowSurface{
		instance: instance,
		vk: &windowSurfaceVk{
			surface: vk.NullSurface,
		},
	}

	surface, err := window.CreateWindowSurface(instance.Handle(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "create window surface")
	}
	w.vk.surface = vk.SurfaceFromPointer(surface)
	//if result := vk.CreateGLFWSurface(w.instance.Handle(), windowHandle, nil, &w.vk.surface); result != vk.Success {
	//return nil, errors.Wrap(vk.Error(result), "create window surface")
	//}

	return &w, nil
}

func (w *WindowSurface) Destroy() {
	if w.vk.surface != vk.NullSurface {
		vk.DestroySurface(w.instance.Handle(), w.vk.surface, nil)
	}
}

func (w *WindowSurface) Handle() vk.Surface {
	return w.vk.surface
}
