package pompeii

import (
	"github.com/pkg/errors"
	vk "github.com/vulkan-go/vulkan"
)

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

func (d *Device) WaitIdle() {
	vk.DeviceWaitIdle(d.logicalDevice)
}

func (d *Device) Handle() vk.Device {
	return d.logicalDevice
}
