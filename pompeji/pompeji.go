package pompeji

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/pkg/errors"
	vk "github.com/vulkan-go/vulkan"
)

func vkString(str string) string {
	if len(str) == 0 {
		return "\x00"
	} else if str[len(str)-1] != '\x00' {
		return str + "\x00"
	}
	return str
}

func Init() error {
	if err := vk.Init(); err != nil {
		return errors.Wrap(err, "could not initialize vulkan")
	}
	return nil
}

func GetAvailableInstanceExtensions() ([]string, error) {
	var count uint32
	if result := vk.EnumerateInstanceExtensionProperties("", &count, nil); result != vk.Success {
		return nil, errors.New("could not count instance extensions")
	}
	extensions := make([]vk.ExtensionProperties, count)
	if result := vk.EnumerateInstanceExtensionProperties("", &count, extensions); result != vk.Success {
		return nil, errors.New("could not get instance extensions")
	}

	names := make([]string, count)
	for t, ext := range extensions {
		ext.Deref()
		names[t] = vk.ToString(ext.ExtensionName[:])
	}
	return names, nil
}

func GetAvailableInstanceLayers() ([]string, error) {
	var count uint32
	if result := vk.EnumerateInstanceLayerProperties(&count, nil); result != vk.Success {
		return nil, errors.New("could not count instance layers")
	}
	layers := make([]vk.LayerProperties, count)
	if result := vk.EnumerateInstanceLayerProperties(&count, layers); result != vk.Success {
		return nil, errors.New("could not get instance layers")
	}

	names := make([]string, count)
	for t, layer := range layers {
		layer.Deref()
		names[t] = vk.ToString(layer.LayerName[:])
	}
	return names, nil
}

type Instance struct {
	instance vk.Instance
	dbg      vk.DebugReportCallback
}

// TODO: Version
func NewInstance(appName, engineName string, layers, extensions []string) (*Instance, error) {
	i := Instance{
		dbg: vk.NullDebugReportCallback,
	}

	for t, name := range layers {
		layers[t] = vkString(name)
	}
	debug := false
	for t, name := range extensions {
		if name == "VK_EXT_debug_report" {
			debug = true
		}
		extensions[t] = vkString(name)
	}

	instanceInfo := vk.InstanceCreateInfo{
		SType: vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: &vk.ApplicationInfo{
			SType:              vk.StructureTypeApplicationInfo,
			PApplicationName:   vkString(appName),
			ApplicationVersion: vk.MakeVersion(1, 0, 0),
			PEngineName:        vkString(engineName),
			EngineVersion:      vk.MakeVersion(0, 0, 1),
			ApiVersion:         vk.ApiVersion10,
		},
		EnabledLayerCount:       uint32(len(layers)),
		PpEnabledLayerNames:     layers,
		EnabledExtensionCount:   uint32(len(extensions)),
		PpEnabledExtensionNames: extensions,
	}

	if result := vk.CreateInstance(&instanceInfo, nil, &i.instance); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "could not create instance")
	}

	vk.InitInstance(i.instance)

	// +Debug
	if debug {
		debugCreateInfo := vk.DebugReportCallbackCreateInfo{
			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit),
			PfnCallback: debugReportCallback,
		}
		if result := vk.CreateDebugReportCallback(i.instance, &debugCreateInfo, nil, &i.dbg); result != vk.Success {
			return nil, errors.Wrap(vk.Error(result), "creating debug report")
		}
	}
	// -Debug

	return &i, nil
}

func (i *Instance) Destroy() {
	if i.dbg != vk.NullDebugReportCallback {
		vk.DestroyDebugReportCallback(i.instance, i.dbg, nil)
	}

	vk.DestroyInstance(i.instance, nil)
}

func debugReportCallback(flags vk.DebugReportFlags, objectType vk.DebugReportObjectType,
	object uint64, location uint, messageCode int32, pLayerPrefix string,
	pMessage string, pUserData unsafe.Pointer) vk.Bool32 {
	switch {
	case flags&vk.DebugReportFlags(vk.DebugReportErrorBit) != 0:
		fmt.Printf("[VK DBG ERR %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	case flags&vk.DebugReportFlags(vk.DebugReportWarningBit) != 0:
		fmt.Printf("[VK DBG WARN %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	default:
		fmt.Printf("[VK DBG UNK] unknown debug message %d (layer %s)", messageCode, pLayerPrefix)
	}
	return vk.Bool32(vk.False)
}

func (i *Instance) EnumerateGPUs() ([]GPU, error) {
	var gpuCount uint32
	if result := vk.EnumeratePhysicalDevices(i.instance, &gpuCount, nil); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "could not count gpus")
	}
	if gpuCount == 0 {
		return nil, errors.New("no valid gpus")
	}
	vkGPUs := make([]vk.PhysicalDevice, gpuCount)
	if result := vk.EnumeratePhysicalDevices(i.instance, &gpuCount, vkGPUs); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "could not enumerate gpus")
	}

	gpus := make([]GPU, gpuCount)
	for t, gpu := range vkGPUs {
		gpus[t] = createGPU(gpu)
	}

	return gpus, nil
}

func (i Instance) Handle() vk.Instance {
	return i.instance
}

type GPUType uint32

const (
	GPUTypeOther      GPUType = GPUType(vk.PhysicalDeviceTypeOther)
	GPUTypeIntegrated         = GPUType(vk.PhysicalDeviceTypeIntegratedGpu)
	GPUTypeDiscrete           = GPUType(vk.PhysicalDeviceTypeDiscreteGpu)
	GPUTypeVirtual            = GPUType(vk.PhysicalDeviceTypeVirtualGpu)
	GPUTypeCPU                = GPUType(vk.PhysicalDeviceTypeCpu)
)

func (g GPUType) String() string {
	switch g {
	case GPUTypeOther:
		return "Other"
	case GPUTypeIntegrated:
		return "Integrated"
	case GPUTypeDiscrete:
		return "Discrete"
	case GPUTypeVirtual:
		return "Virtual"
	case GPUTypeCPU:
		return "CPU"
	default:
		panic("unreachable")
	}
}

type GPU struct {
	Name string
	Type GPUType

	device   vk.PhysicalDevice
	props    vk.PhysicalDeviceProperties
	memProps vk.PhysicalDeviceMemoryProperties
	features vk.PhysicalDeviceFeatures
}

func createGPU(device vk.PhysicalDevice) GPU {
	g := GPU{
		device: device,
	}

	vk.GetPhysicalDeviceProperties(g.device, &g.props)
	g.props.Deref()
	g.props.Limits.Deref()
	g.props.SparseProperties.Deref()

	vk.GetPhysicalDeviceMemoryProperties(g.device, &g.memProps)
	g.memProps.Deref()

	vk.GetPhysicalDeviceFeatures(g.device, &g.features)
	g.features.Deref()

	g.Name = string(g.props.DeviceName[:])
	g.Type = GPUType(g.props.DeviceType)

	return g
}

func (g GPU) Debug() string {
	buffer := bytes.Buffer{}

	buffer.WriteString(fmt.Sprintln("Device Name:", g.Name))
	buffer.WriteString(fmt.Sprintln("Device Type:", g.Type))
	buffer.WriteString("## Backend\n")
	buffer.WriteString(fmt.Sprintf("Vulkan v%d.%d.%d\n",
		(g.props.ApiVersion>>22)&0x3ff,
		(g.props.ApiVersion>>12)&0x3ff,
		g.props.ApiVersion&0xfff,
	))
	buffer.WriteString(fmt.Sprintf("Driver v%d.%d.%d\n",
		(g.props.DriverVersion>>22)&0x3ff,
		(g.props.DriverVersion>>12)&0x3ff,
		g.props.DriverVersion&0xfff,
	))
	buffer.WriteString(fmt.Sprintln("Max Image Dimension:", g.props.Limits.MaxImageDimension2D))
	buffer.WriteString(fmt.Sprintln("Max Viewports:", g.props.Limits.MaxViewports))
	buffer.WriteString(fmt.Sprintln("Max Viewport Dimensions:", g.props.Limits.MaxViewportDimensions[0], g.props.Limits.MaxViewportDimensions[1]))

	return buffer.String()
}

func (g GPU) Match(resWidth, resHeight uint32) bool {
	return (g.props.Limits.MaxViewportDimensions[0] >= resWidth && g.props.Limits.MaxViewportDimensions[1] >= resHeight)
}

func (g GPU) Handle() vk.PhysicalDevice {
	return g.device
}

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

func (w WindowSurface) Handle() vk.Surface {
	return w.vk.surface
}