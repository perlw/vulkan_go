package main

import (
	"fmt"
	"runtime"

	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

func init() {
	runtime.LockOSThread()
	//runtime.GOMAXPROCS(2)
}

func main() {
	glfw.Init()
	defer glfw.Terminate()
	if err := vk.Init(); err != nil {
		fmt.Println("err:", err.Error())
		return
	}

	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	window, err := glfw.CreateWindow(640, 480, "GLFW: Abyssal Drifter", nil, nil)
	if err != nil {
		panic(err.Error())
	}
	defer window.Destroy()

	// Set up Vulkan instance
	exts := vk.GetRequiredInstanceExtensions()
	//exts = append(exts, "VK_EXT_debug_report")

	instanceInfo := vk.InstanceCreateInfo{
		SType: vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: &vk.ApplicationInfo{
			SType:              vk.StructureTypeApplicationInfo,
			PApplicationName:   "Abyssal Drifter\x00",
			ApplicationVersion: vk.MakeVersion(1, 0, 0),
			PEngineName:        "MYRLE\x00",
			EngineVersion:      vk.MakeVersion(0, 0, 1),
			ApiVersion:         vk.ApiVersion10,
		},
		EnabledLayerCount:       0,
		PpEnabledLayerNames:     nil,
		EnabledExtensionCount:   uint32(len(exts)),
		PpEnabledExtensionNames: exts,
	}

	var instance vk.Instance
	if result := vk.CreateInstance(&instanceInfo, nil, &instance); result != vk.Success {
		fmt.Println("err:", "instance", result)
		return
	}
	defer vk.DestroyInstance(instance, nil)

	fmt.Println("instance created, exts:", exts)

	vk.InitInstance(instance)

	// Enumerate GPUs
	var gpuCount uint32
	if result := vk.EnumeratePhysicalDevices(instance, &gpuCount, nil); result != vk.Success {
		fmt.Println("err:", "count devices", result)
		return
	}
	if gpuCount == 0 {
		fmt.Println("err: no gpus")
		return
	}
	fmt.Println("GPUS:", gpuCount)
	gpus := make([]vk.PhysicalDevice, gpuCount)
	if result := vk.EnumeratePhysicalDevices(instance, &gpuCount, gpus); result != vk.Success {
		fmt.Println("err:", "enumerate devices", result)
		return
	}
	gpu := gpus[0]

	// Get properties
	var gpuProperties vk.PhysicalDeviceProperties
	var memoryProperties vk.PhysicalDeviceMemoryProperties
	var gpuFeatures vk.PhysicalDeviceFeatures
	vk.GetPhysicalDeviceProperties(gpu, &gpuProperties)
	gpuProperties.Deref()
	vk.GetPhysicalDeviceMemoryProperties(gpu, &memoryProperties)
	memoryProperties.Deref()
	vk.GetPhysicalDeviceFeatures(gpu, &gpuFeatures)
	gpuFeatures.Deref()

	fmt.Printf("Vulkan v%d.%d.%d\n",
		(gpuProperties.ApiVersion>>22)&0x3ff,
		(gpuProperties.ApiVersion>>12)&0x3ff,
		gpuProperties.ApiVersion&0xfff,
	)
	fmt.Println("Device Name:", string(gpuProperties.DeviceName[:]))
	fmt.Println("Device Type:", gpuProperties.DeviceType)
	fmt.Printf("Driver v%d.%d.%d\n",
		(gpuProperties.DriverVersion>>22)&0x3ff,
		(gpuProperties.DriverVersion>>12)&0x3ff,
		gpuProperties.DriverVersion&0xfff,
	)
	gpuProperties.Limits.Deref()
	fmt.Println("Max Image Dimension:", gpuProperties.Limits.MaxImageDimension2D)

	// Check queue families
	var queueFamilyCount uint32
	vk.GetPhysicalDeviceQueueFamilyProperties(gpu, &queueFamilyCount, nil)
	if queueFamilyCount == 0 {
		fmt.Println("err: no queue families")
		return
	}
	fmt.Println("Queue Families:", queueFamilyCount)
	queueFamilies := make([]vk.QueueFamilyProperties, queueFamilyCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(gpu, &queueFamilyCount, queueFamilies)
	var graphicsFamilyIndex uint32
	for i, family := range queueFamilies {
		family.Deref()
		if family.QueueCount > 0 && family.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0 {
			graphicsFamilyIndex = uint32(i)
		}
		if family.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0 {
			fmt.Println("family:", i, "graphics")
		}
		if family.QueueFlags&vk.QueueFlags(vk.QueueComputeBit) != 0 {
			fmt.Println("family:", i, "compute")
		}
		if family.QueueFlags&vk.QueueFlags(vk.QueueTransferBit) != 0 {
			fmt.Println("family:", i, "transfer")
		}
	}
	fmt.Println("Graphics family index:", graphicsFamilyIndex)

	// Create device
	queuePriorities := []float32{1.0}
	deviceCreateInfo := vk.DeviceCreateInfo{
		SType:                vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount: 1,
		PQueueCreateInfos: []vk.DeviceQueueCreateInfo{
			{
				SType:            vk.StructureTypeDeviceQueueCreateInfo,
				QueueFamilyIndex: graphicsFamilyIndex,
				QueueCount:       uint32(len(queuePriorities)),
				PQueuePriorities: queuePriorities,
			},
		},
		EnabledLayerCount:       0,
		PpEnabledLayerNames:     nil,
		EnabledExtensionCount:   1,
		PpEnabledExtensionNames: []string{"VK_KHR_swapchain\x00"},
	}
	var device vk.Device
	if result := vk.CreateDevice(gpu, &deviceCreateInfo, nil, &device); result != vk.Success {
		fmt.Println("err:", "create device", result)
		return
	}
	defer (func() {
		vk.DeviceWaitIdle(device)
		vk.DestroyDevice(device, nil)
	})()

	// Get command queue
	var cmdQueue vk.Queue
	vk.GetDeviceQueue(device, graphicsFamilyIndex, 0, &cmdQueue)

	// Semaphores
	var imageAvailableSemaphore vk.Semaphore
	var renderingFinishedSemaphore vk.Semaphore
	semaphoreCreateInfo := vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	if result := vk.CreateSemaphore(device, &semaphoreCreateInfo, nil, &imageAvailableSemaphore); result != vk.Success {
		fmt.Println("err:", "create semaphore, image", result)
		return
	}
	if result := vk.CreateSemaphore(device, &semaphoreCreateInfo, nil, &renderingFinishedSemaphore); result != vk.Success {
		fmt.Println("err:", "create semaphore, rendering", result)
		return
	}

	// Surface
	var surface vk.Surface
	if result := vk.CreateWindowSurface(instance, window.GLFWWindow(), nil, &surface); result != vk.Success {
		fmt.Println("err:", "create window surface", result)
		return
	}
	defer vk.DestroySurface(instance, surface, nil)

	// Swap chain
	var surfaceCapabilities vk.SurfaceCapabilities
	if result := vk.GetPhysicalDeviceSurfaceCapabilities(gpu, surface, &surfaceCapabilities); result != vk.Success {
		fmt.Println("err:", "get surface caps", result)
		return
	}
	surfaceCapabilities.Deref()
	surfaceCapabilities.MinImageExtent.Deref()
	surfaceCapabilities.MaxImageExtent.Deref()

	fmt.Println("surface min:", surfaceCapabilities.MinImageExtent.Width, surfaceCapabilities.MinImageExtent.Height)
	fmt.Println("surface max:", surfaceCapabilities.MaxImageExtent.Width, surfaceCapabilities.MaxImageExtent.Height)

	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(gpu, surface, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(gpu, surface, &formatCount, formats)
	format := formats[0]
	format.Deref()

	var oldSwapchain vk.Swapchain
	var swapchain vk.Swapchain
	swapChainCreateInfo := vk.SwapchainCreateInfo{
		SType:           vk.StructureTypeSwapchainCreateInfo,
		Surface:         surface,
		MinImageCount:   2,
		ImageFormat:     format.Format,
		ImageColorSpace: format.ColorSpace,
		ImageExtent: vk.Extent2D{
			Width:  640,
			Height: 480,
		},
		ImageArrayLayers:      1,
		ImageUsage:            vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit | vk.ImageUsageTransferDstBit),
		ImageSharingMode:      vk.SharingModeExclusive,
		QueueFamilyIndexCount: 0,
		PreTransform:          vk.SurfaceTransformIdentityBit,
		CompositeAlpha:        vk.CompositeAlphaOpaqueBit,
		PresentMode:           vk.PresentModeFifo,
		Clipped:               vk.True,
		OldSwapchain:          oldSwapchain,
	}
	if result := vk.CreateSwapchain(device, &swapChainCreateInfo, nil, &swapchain); result != vk.Success {
		fmt.Println("err:", "create swapchain", result)
		return
	}
	if oldSwapchain != vk.NullSwapchain {
		vk.DestroySwapchain(device, oldSwapchain, nil)
	}

	for !window.ShouldClose() {
		var imageIndex uint32
		result := vk.AcquireNextImage(device, swapchain, vk.MaxUint64, imageAvailableSemaphore, vk.NullFence, &imageIndex)
		switch result {
		case vk.Success:
			fallthrough
		case vk.Suboptimal:
		case vk.ErrorOutOfDate:
			fmt.Println("outdate")
		default:
			fmt.Println("err:", "aquire image", result)
			return
		}

		submitInfo := vk.SubmitInfo{
			SType:              vk.StructureTypeSubmitInfo,
			WaitSemaphoreCount: 1,
			PWaitSemaphores: []vk.Semaphore{
				imageAvailableSemaphore,
			},
			PWaitDstStageMask: []vk.PipelineStageFlags{
				vk.PipelineStageFlags(vk.PipelineStageTransferBit),
			},
			CommandBufferCount: 1,
			PCommandBuffers:    []vk.CommandBuffer{
				// Missing command buffers
			},
			SignalSemaphoreCount: 1,
			PSignalSemaphores: []vk.Semaphore{
				renderingFinishedSemaphore,
			},
		}
		if result := vk.QueueSubmit(nil /* present queue */, 1, []vk.SubmitInfo{
			submitInfo,
		}, vk.NullFence); result != vk.Success {
			fmt.Println("err:", "queue submit", result)
			return
		}

		glfw.PollEvents()
	}

	fmt.Println("fin")
}
