package pompeii

import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/pkg/errors"
	vk "github.com/vulkan-go/vulkan"
)

func inStringSlice(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

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

func getAvailableInstanceExtensions() ([]string, error) {
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

func getAvailableInstanceLayers() ([]string, error) {
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

	activeLayers := []string{}
	if layers != nil {
		available, err := getAvailableInstanceLayers()
		if err != nil {
			return nil, errors.Wrap(err, "could not get layers")
		}
		for _, name := range layers {
			if inStringSlice(available, name) {
				activeLayers = append(activeLayers, vkString(name))
			}
		}
	}

	debug := false
	activeExtensions := vk.GetRequiredInstanceExtensions()
	if extensions != nil {
		available, err := getAvailableInstanceExtensions()
		if err != nil {
			return nil, errors.Wrap(err, "could not get instance extensions")
		}
		for _, name := range extensions {
			if inStringSlice(available, name) {
				if name == "VK_EXT_debug_report" {
					debug = true
				}
				activeExtensions = append(activeExtensions, vkString(name))
			}
		}
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
		EnabledLayerCount:       uint32(len(activeLayers)),
		PpEnabledLayerNames:     activeLayers,
		EnabledExtensionCount:   uint32(len(activeExtensions)),
		PpEnabledExtensionNames: activeExtensions,
	}

	if result := vk.CreateInstance(&instanceInfo, nil, &i.instance); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "could not create instance")
	}

	vk.InitInstance(i.instance)

	fmt.Printf("instance created;\n\tlayers: %v\n\texts: %v\n", activeLayers, activeExtensions)

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
		gpus[t] = newGPU(gpu)
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

type QueueFamily struct {
	Index    int
	Graphics bool
	Compute  bool
	Transfer bool

	physicalDevice vk.PhysicalDevice
}

func (q QueueFamily) SurfacePresentSupport(surface Surface) bool {
	var presentSupport vk.Bool32
	vk.GetPhysicalDeviceSurfaceSupport(q.physicalDevice, uint32(q.Index), surface.Handle(), &presentSupport)
	return (presentSupport > 0)
}

type GPU struct {
	Name string
	Type GPUType

	physicalDevice vk.PhysicalDevice
	props          vk.PhysicalDeviceProperties
	memProps       vk.PhysicalDeviceMemoryProperties
	features       vk.PhysicalDeviceFeatures
}

func newGPU(physicalDevice vk.PhysicalDevice) GPU {
	g := GPU{
		physicalDevice: physicalDevice,
	}

	vk.GetPhysicalDeviceProperties(g.physicalDevice, &g.props)
	g.props.Deref()
	g.props.Limits.Deref()
	g.props.SparseProperties.Deref()

	vk.GetPhysicalDeviceMemoryProperties(g.physicalDevice, &g.memProps)
	g.memProps.Deref()

	vk.GetPhysicalDeviceFeatures(g.physicalDevice, &g.features)
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

func (g GPU) QueueFamilies() ([]QueueFamily, error) {
	var queueFamilyCount uint32
	vk.GetPhysicalDeviceQueueFamilyProperties(g.physicalDevice, &queueFamilyCount, nil)
	if queueFamilyCount == 0 {
		return nil, errors.New("no queue families")
	}

	families := []QueueFamily{}

	queueFamilies := make([]vk.QueueFamilyProperties, queueFamilyCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(g.physicalDevice, &queueFamilyCount, queueFamilies)
	for i, family := range queueFamilies {
		family.Deref()

		families = append(families, QueueFamily{
			Index:          i,
			Graphics:       (family.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0),
			Compute:        (family.QueueFlags&vk.QueueFlags(vk.QueueComputeBit) != 0),
			Transfer:       (family.QueueFlags&vk.QueueFlags(vk.QueueTransferBit) != 0),
			physicalDevice: g.physicalDevice,
		})
	}

	return families, nil
}

func (g GPU) Handle() vk.PhysicalDevice {
	return g.physicalDevice
}

type Device struct {
	GraphicsIndex int
	PresentIndex  int

	logicalDevice vk.Device
}

func NewDevice(g *GPU, graphicsFamilyIndex, presentFamilyIndex int) (*Device, error) {
	d := Device{
		GraphicsIndex: graphicsFamilyIndex,
		PresentIndex:  presentFamilyIndex,
	}

	queuePriorities := []float32{1.0}
	deviceCreateInfo := vk.DeviceCreateInfo{
		SType:                vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount: 1,
		PQueueCreateInfos: []vk.DeviceQueueCreateInfo{
			{
				SType:            vk.StructureTypeDeviceQueueCreateInfo,
				QueueFamilyIndex: uint32(graphicsFamilyIndex),
				QueueCount:       uint32(len(queuePriorities)),
				PQueuePriorities: queuePriorities,
			},
		},
		EnabledLayerCount:       0,
		PpEnabledLayerNames:     nil,
		EnabledExtensionCount:   1,
		PpEnabledExtensionNames: []string{vkString("VK_KHR_swapchain")},
	}
	if result := vk.CreateDevice(g.Handle(), &deviceCreateInfo, nil, &d.logicalDevice); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "create device")
	}

	return &d, nil
}

func (d *Device) Destroy() {
	d.WaitIdle()
	vk.DestroyDevice(d.logicalDevice, nil)
}

func (d Device) WaitIdle() {
	vk.DeviceWaitIdle(d.logicalDevice)
}

func (d Device) Handle() vk.Device {
	return d.logicalDevice
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
