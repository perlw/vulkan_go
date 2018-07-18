package pompeii

import (
	"github.com/pkg/errors"
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

func NewWindowSurface(instance *Instance, windowHandle uintptr) (*WindowSurface, error) {
	w := WindowSurface{
		instance: instance,
		vk: &windowSurfaceVk{
			surface: vk.NullSurface,
		},
	}

	if result := vk.CreateWindowSurface(w.instance.Handle(), windowHandle, nil, &w.vk.surface); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "create window surface")
	}

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
