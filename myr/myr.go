package myr

import (
	"github.com/pkg/errors"
	"github.com/vulkan-go/glfw/v3.3/glfw"

	"github.com/perlw/abyssal_drifter/logger"
	"github.com/perlw/abyssal_drifter/pompeji"
)

const engineName = "MYR"

type Myr struct {
	log logger.Logger

	window   *glfw.Window
	instance *pompeji.Instance
	gpu      *pompeji.GPU
	surface  pompeji.Surface
	device   *pompeji.Device
}

func New(appName string, resWidth, resHeight int) (*Myr, error) {
	m := Myr{
		log: logger.New(engineName),
	}

	glfw.Init()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	var err error
	m.window, err = glfw.CreateWindow(resWidth, resHeight, appName, nil, nil)
	if err != nil {
		panic(err.Error())
	}

	if err := pompeji.Init(); err != nil {
		return nil, err
	}

	m.instance, err = pompeji.NewInstance(appName, engineName, []string{
		"VK_LAYER_LUNARG_standard_validation",
	}, []string{
		"VK_EXT_debug_report",
	})
	if err != nil {
		return nil, err
	}

	gpus, err := m.instance.EnumerateGPUs()
	if err != nil {
		return nil, err
	}
	for t, gpu := range gpus {
		m.log.Log("# GPU %d\n%s", t, gpu.Debug())
		m.log.Log(gpu.Debug())
		if gpu.Match(uint32(resWidth), uint32(resHeight)) {
			m.gpu = &gpus[t]
		}
	}
	if m.gpu == nil {
		return nil, errors.New("no matching GPU")
	}
	m.log.Log("Picked: %s\n", m.gpu.Name)

	m.surface, err = pompeji.NewWindowSurface(m.instance, m.window.GLFWWindow())
	if err != nil {
		return nil, err
	}

	families, err := m.gpu.QueueFamilies()
	if err != nil {
		return nil, errors.Wrap(err, "could not get families")
	}
	m.log.Log("Queue families: %d\n", len(families))
	graphicsFamily := -1
	presentFamily := -1
	for _, family := range families {
		m.log.Log("%+v\n", family)
		if family.Graphics {
			graphicsFamily = family.Index
			if family.SurfacePresentSupport(m.surface) {
				m.log.Log("Family %d => present support\n", family.Index)
				presentFamily = family.Index
			}
		}
	}
	m.device, err = m.gpu.CreateDevice(graphicsFamily, presentFamily)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *Myr) Destroy() {
	m.device.Destroy()
	m.surface.Destroy()

	m.instance.Destroy()
	m.window.Destroy()

	glfw.Terminate()
}

func (m Myr) ShouldClose() bool {
	return m.window.ShouldClose()
}

func (m Myr) BackendInstance() *pompeji.Instance {
	return m.instance
}

func (m Myr) BackendGPU() *pompeji.GPU {
	return m.gpu
}

func (m Myr) BackendSurface() pompeji.Surface {
	return m.surface
}

func (m Myr) BackendDevice() *pompeji.Device {
	return m.device
}
