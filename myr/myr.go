package myr

import (
	"github.com/pkg/errors"
	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"

	"github.com/perlw/abyssal_drifter/logger"
	"github.com/perlw/abyssal_drifter/pompeji"
)

const engineName = "MYR"

func inStringSlice(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

type Myr struct {
	log logger.Logger

	window   *glfw.Window
	instance *pompeji.Instance
	gpu      *pompeji.GPU
	surface  pompeji.Surface
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

	layers := []string{}
	{
		available, err := pompeji.GetAvailableInstanceLayers()
		if err != nil {
			return nil, errors.Wrap(err, "could not get layers")
		}
		if inStringSlice(available, "VK_LAYER_LUNARG_standard_validation") {
			layers = append(layers, "VK_LAYER_LUNARG_standard_validation")
		}
	}

	exts := vk.GetRequiredInstanceExtensions()
	{
		available, err := pompeji.GetAvailableInstanceExtensions()
		if err != nil {
			return nil, errors.Wrap(err, "could not get instance extensions")
		}
		if inStringSlice(available, "VK_EXT_debug_report") {
			exts = append(exts, "VK_EXT_debug_report")
		}
	}

	m.instance, err = pompeji.NewInstance(appName, engineName, layers, exts)
	if err != nil {
		return nil, err
	}

	m.log.Log("instance created;\n\tlayers: %v\n\texts: %v\n", layers, exts)

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

	return &m, nil
}

func (m *Myr) Destroy() {
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
