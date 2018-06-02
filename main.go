package main

import (
	"fmt"
	"runtime"
	"unsafe"

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

	// Surface
	var surface vk.Surface
	if result := vk.CreateWindowSurface(instance, window.GLFWWindow(), nil, &surface); result != vk.Success {
		fmt.Println("err:", "create window surface", result)
		return
	}
	defer vk.DestroySurface(instance, surface, nil)

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
	var presentFamilyIndex uint32
	for i, family := range queueFamilies {
		family.Deref()

		var presentSupport vk.Bool32
		vk.GetPhysicalDeviceSurfaceSupport(gpu, uint32(i), surface, &presentSupport)

		if family.QueueCount > 0 && family.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0 {
			graphicsFamilyIndex = uint32(i)
			if presentSupport > 0 {
				fmt.Println("Yes! Present support")
				presentFamilyIndex = uint32(i)
			}
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
	fmt.Println("Present family index:", presentFamilyIndex)

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
	var graphicsQueue vk.Queue
	var presentQueue vk.Queue
	vk.GetDeviceQueue(device, graphicsFamilyIndex, 0, &graphicsQueue)
	vk.GetDeviceQueue(device, presentFamilyIndex, 0, &presentQueue)

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
	defer vk.DestroySemaphore(device, imageAvailableSemaphore, nil)
	defer vk.DestroySemaphore(device, renderingFinishedSemaphore, nil)

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
	defer vk.DestroySwapchain(device, swapchain, nil)

	// Command queue buffer memory pool
	var presentQueueCmdPool vk.CommandPool
	cmdPoolCreateInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: presentFamilyIndex,
	}
	if result := vk.CreateCommandPool(device, &cmdPoolCreateInfo, nil, &presentQueueCmdPool); result != vk.Success {
		fmt.Println("err:", "create command pool", result)
		return
	}
	defer vk.DestroyCommandPool(device, presentQueueCmdPool, nil)

	// Set up Command buffers
	var imageCount uint32
	if result := vk.GetSwapchainImages(device, swapchain, &imageCount, nil); result != vk.Success {
		fmt.Println("err:", "get swapchain image count", result)
		return
	}
	fmt.Println("Swapchain image count:", imageCount)
	presentQueueCmdBuffers := make([]vk.CommandBuffer, imageCount)
	cmdBufferAllocateInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        presentQueueCmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: imageCount,
	}
	if result := vk.AllocateCommandBuffers(device, &cmdBufferAllocateInfo, presentQueueCmdBuffers); result != vk.Success {
		fmt.Println("err:", "allocate command buffers", result)
		return
	}
	defer vk.FreeCommandBuffers(device, presentQueueCmdPool, imageCount, presentQueueCmdBuffers)

	// Record command buffers
	swapChainImages := make([]vk.Image, imageCount)
	if result := vk.GetSwapchainImages(device, swapchain, &imageCount, swapChainImages); result != vk.Success {
		fmt.Println("err:", "get swapchain images", result)
		return
	}
	cmdBufferBeginInfo := vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit),
	}
	clearColor := (func(r, g, b, a float32) vk.ClearColorValue {
		var vkValue vk.ClearColorValue
		clearColor := (*[4]float32)(unsafe.Pointer(&vkValue))
		clearColor[0] = r
		clearColor[1] = g
		clearColor[2] = b
		clearColor[3] = a
		return vkValue
	})(0.5, 0.5, 1.0, 0.0)
	imageSubresourceRange := vk.ImageSubresourceRange{
		AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
		LevelCount: 1,
		LayerCount: 1,
	}
	for i := range swapChainImages {
		barrierFromPresentToClear := vk.ImageMemoryBarrier{
			SType:               vk.StructureTypeImageMemoryBarrier,
			SrcAccessMask:       vk.AccessFlags(vk.AccessMemoryReadBit),
			DstAccessMask:       vk.AccessFlags(vk.AccessTransferWriteBit),
			OldLayout:           vk.ImageLayoutUndefined,
			NewLayout:           vk.ImageLayoutTransferDstOptimal,
			SrcQueueFamilyIndex: presentFamilyIndex,
			DstQueueFamilyIndex: presentFamilyIndex,
			Image:               swapChainImages[i],
			SubresourceRange:    imageSubresourceRange,
		}
		barrierFromClearToPresent := vk.ImageMemoryBarrier{
			SType:               vk.StructureTypeImageMemoryBarrier,
			SrcAccessMask:       vk.AccessFlags(vk.AccessTransferWriteBit),
			DstAccessMask:       vk.AccessFlags(vk.AccessMemoryReadBit),
			OldLayout:           vk.ImageLayoutTransferDstOptimal,
			NewLayout:           vk.ImageLayoutPresentSrc,
			SrcQueueFamilyIndex: presentFamilyIndex,
			DstQueueFamilyIndex: presentFamilyIndex,
			Image:               swapChainImages[i],
			SubresourceRange:    imageSubresourceRange,
		}

		vk.BeginCommandBuffer(presentQueueCmdBuffers[i], &cmdBufferBeginInfo)
		vk.CmdPipelineBarrier(presentQueueCmdBuffers[i], vk.PipelineStageFlags(vk.PipelineStageTransferBit), vk.PipelineStageFlags(vk.PipelineStageTransferBit), 0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{barrierFromPresentToClear})
		vk.CmdClearColorImage(presentQueueCmdBuffers[i], swapChainImages[i], vk.ImageLayoutTransferDstOptimal, &clearColor, 1, []vk.ImageSubresourceRange{imageSubresourceRange})
		vk.CmdPipelineBarrier(presentQueueCmdBuffers[i], vk.PipelineStageFlags(vk.PipelineStageTransferBit), vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit), 0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{barrierFromClearToPresent})
		if result := vk.EndCommandBuffer(presentQueueCmdBuffers[i]); result != vk.Success {
			fmt.Println("err:", "record command buffer", i, ":", result)
			return
		}
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
			PCommandBuffers: []vk.CommandBuffer{
				presentQueueCmdBuffers[imageIndex],
			},
			SignalSemaphoreCount: 1,
			PSignalSemaphores: []vk.Semaphore{
				renderingFinishedSemaphore,
			},
		}
		if result := vk.QueueSubmit(presentQueue, 1, []vk.SubmitInfo{
			submitInfo,
		}, vk.NullFence); result != vk.Success {
			fmt.Println("err:", "queue submit", result)
			return
		}

		presentInfo := vk.PresentInfo{
			SType:              vk.StructureTypePresentInfo,
			WaitSemaphoreCount: 1,
			PWaitSemaphores: []vk.Semaphore{
				renderingFinishedSemaphore,
			},
			SwapchainCount: 1,
			PSwapchains: []vk.Swapchain{
				swapchain,
			},
			PImageIndices: []uint32{
				imageIndex,
			},
		}
		result = vk.QueuePresent(presentQueue, &presentInfo)
		switch result {
		case vk.Success:
			break
		case vk.Suboptimal:
			fallthrough
		case vk.ErrorOutOfDate:
			fmt.Println("outdate")
		default:
			fmt.Println("err:", "image present", result)
			return
		}

		glfw.PollEvents()
	}

	vk.DeviceWaitIdle(device)
	fmt.Println("fin")
}
