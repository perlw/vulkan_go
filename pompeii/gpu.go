package pompeii

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	vk "github.com/vulkan-go/vulkan"
)

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

func (q *QueueFamily) SurfacePresentSupport(surface Surface) bool {
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

func (g *GPU) Debug() string {
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

func (g *GPU) Match(resWidth, resHeight uint32) bool {
	return (g.props.Limits.MaxViewportDimensions[0] >= resWidth && g.props.Limits.MaxViewportDimensions[1] >= resHeight)
}

func (g *GPU) QueueFamilies() ([]QueueFamily, error) {
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

func (g *GPU) Handle() vk.PhysicalDevice {
	return g.physicalDevice
}
