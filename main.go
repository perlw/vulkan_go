package main

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"unsafe"

	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"

	"github.com/perlw/abyssal_drifter/logger"
	"github.com/perlw/abyssal_drifter/myr"
	"github.com/perlw/abyssal_drifter/pompeii"
)

func init() {
	runtime.LockOSThread()
}

// +Byte slice to uint32 slice
type sliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

func sliceUint32(data []byte) []uint32 {
	const m = 0x7fffffff
	return (*[m / 4]uint32)(unsafe.Pointer((*sliceHeader)(unsafe.Pointer(&data)).Data))[:len(data)/4]
}

// -Byte slice to uint32 slice

func vkString(str string) string {
	if len(str) == 0 {
		return "\x00"
	} else if str[len(str)-1] != '\x00' {
		return str + "\x00"
	}
	return str
}

const AppName = "Abyssal Drifter"
const ResWidth = 640
const ResHeight = 480

func main() {
	log := logger.New(AppName)

	framework, err := myr.New(AppName, ResWidth, ResHeight)
	if err != nil {
		panic(err.Error())
	}
	defer framework.Destroy()

	// NOTE: Only for dev
	gpuHandle := framework.BackendGPU().Handle()
	surfaceHandle := framework.BackendSurface().Handle()
	device := framework.BackendDevice()
	deviceHandle := framework.BackendDevice().Handle()

	// +Prepare rendering
	// Get command queue
	var graphicsQueue vk.Queue
	var presentQueue vk.Queue
	vk.GetDeviceQueue(deviceHandle, uint32(device.GraphicsIndex), 0, &graphicsQueue)
	vk.GetDeviceQueue(deviceHandle, uint32(device.PresentIndex), 0, &presentQueue)

	// Semaphores
	imageAvailableSemaphore, err := pompeii.NewSemaphore(device)
	if err != nil {
		log.Err(err, "image")
		return
	}
	renderingFinishedSemaphore, err := pompeii.NewSemaphore(device)
	if err != nil {
		log.Err(err, "rendering")
		return
	}
	defer imageAvailableSemaphore.Destroy()
	defer renderingFinishedSemaphore.Destroy()

	// Swap chain
	var surfaceCapabilities vk.SurfaceCapabilities
	if result := vk.GetPhysicalDeviceSurfaceCapabilities(gpuHandle, surfaceHandle, &surfaceCapabilities); result != vk.Success {
		log.Err(vk.Error(result), "get surface caps")
		return
	}
	surfaceCapabilities.Deref()
	surfaceCapabilities.MinImageExtent.Deref()
	surfaceCapabilities.MaxImageExtent.Deref()

	log.Log("surface min: %dx%d", surfaceCapabilities.MinImageExtent.Width, surfaceCapabilities.MinImageExtent.Height)
	log.Log("surface max: %dx%d", surfaceCapabilities.MaxImageExtent.Width, surfaceCapabilities.MaxImageExtent.Height)

	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(gpuHandle, surfaceHandle, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(gpuHandle, surfaceHandle, &formatCount, formats)
	format := formats[0]
	format.Deref()

	var oldSwapchain vk.Swapchain
	var swapchain vk.Swapchain
	swapChainCreateInfo := vk.SwapchainCreateInfo{
		SType:           vk.StructureTypeSwapchainCreateInfo,
		Surface:         surfaceHandle,
		MinImageCount:   2,
		ImageFormat:     format.Format,
		ImageColorSpace: format.ColorSpace,
		ImageExtent: vk.Extent2D{
			Width:  ResWidth,
			Height: ResHeight,
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
	if result := vk.CreateSwapchain(deviceHandle, &swapChainCreateInfo, nil, &swapchain); result != vk.Success {
		log.Err(vk.Error(result), "create swapchain")
		return
	}
	if oldSwapchain != vk.NullSwapchain {
		vk.DestroySwapchain(deviceHandle, oldSwapchain, nil)
	}
	defer vk.DestroySwapchain(deviceHandle, swapchain, nil)
	// -Prepare rendering

	// +Set up render pass
	// Creating render pass
	attachmentDescriptions := []vk.AttachmentDescription{
		{
			Format:         format.Format,
			Samples:        vk.SampleCount1Bit,
			LoadOp:         vk.AttachmentLoadOpClear,
			StoreOp:        vk.AttachmentStoreOpStore,
			StencilLoadOp:  vk.AttachmentLoadOpDontCare,
			StencilStoreOp: vk.AttachmentStoreOpDontCare,
			InitialLayout:  vk.ImageLayoutPresentSrc,
			FinalLayout:    vk.ImageLayoutPresentSrc,
		},
	}
	colorAttachmentReferences := []vk.AttachmentReference{
		{
			Layout: vk.ImageLayoutColorAttachmentOptimal,
		},
	}
	subpassDescriptions := []vk.SubpassDescription{
		{
			PipelineBindPoint:    vk.PipelineBindPointGraphics,
			ColorAttachmentCount: 1,
			PColorAttachments:    colorAttachmentReferences,
		},
	}

	renderPassCreateInfo := vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: 1,
		PAttachments:    attachmentDescriptions,
		SubpassCount:    1,
		PSubpasses:      subpassDescriptions,
	}
	var renderPass vk.RenderPass
	if result := vk.CreateRenderPass(deviceHandle, &renderPassCreateInfo, nil, &renderPass); result != vk.Success {
		log.Err(vk.Error(result), "create render pass")
		return
	}
	defer vk.DestroyRenderPass(deviceHandle, renderPass, nil)

	// Creating framebuffers
	var imageCount uint32
	if result := vk.GetSwapchainImages(deviceHandle, swapchain, &imageCount, nil); result != vk.Success {
		log.Err(vk.Error(result), "get swapchain image count")
		return
	}
	log.Log("Swapchain image count: %d", imageCount)

	swapChainImages := make([]vk.Image, imageCount)
	if result := vk.GetSwapchainImages(deviceHandle, swapchain, &imageCount, swapChainImages); result != vk.Success {
		log.Err(vk.Error(result), "get swapchain images")
		return
	}

	// TODO: Use single framebuffer, render to texture, then make swapchain copy from texture
	framebufferWidth := uint32(ResWidth)
	framebufferHeight := uint32(ResHeight)
	framebuffers := make([]vk.Framebuffer, len(swapChainImages))
	framebufferViews := make([]vk.ImageView, len(swapChainImages))
	for i, img := range swapChainImages {
		imageViewCreateInfo := vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			Image:    img,
			ViewType: vk.ImageViewType2d,
			Format:   format.Format,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleIdentity,
				G: vk.ComponentSwizzleIdentity,
				B: vk.ComponentSwizzleIdentity,
				A: vk.ComponentSwizzleIdentity,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
				BaseMipLevel:   0,
				LevelCount:     1,
				BaseArrayLayer: 0,
				LayerCount:     1,
			},
		}
		if result := vk.CreateImageView(deviceHandle, &imageViewCreateInfo, nil, &framebufferViews[i]); result != vk.Success {
			log.Err(vk.Error(result), "create image view")
			return
		}

		// Framebuffer parameters
		framebufferCreateInfo := vk.FramebufferCreateInfo{
			SType:           vk.StructureTypeFramebufferCreateInfo,
			RenderPass:      renderPass,
			AttachmentCount: 1,
			PAttachments: []vk.ImageView{
				framebufferViews[i],
			},
			Width:  framebufferWidth,
			Height: framebufferHeight,
			Layers: 1,
		}
		if result := vk.CreateFramebuffer(deviceHandle, &framebufferCreateInfo, nil, &framebuffers[i]); result != vk.Success {
			log.Err(vk.Error(result), "create framebuffer")
			return
		}
	}

	defer (func() {
		for i := range swapChainImages {
			vk.DestroyImageView(deviceHandle, framebufferViews[i], nil)
			vk.DestroyFramebuffer(deviceHandle, framebuffers[i], nil)
		}
	})()

	// Shaders
	var vertShaderModule vk.ShaderModule
	var fragShaderModule vk.ShaderModule
	{
		shaderCode, err := ioutil.ReadFile("tri.vert.spv")
		if err != nil {
			log.Err(err, "read vertex code")
			return
		}

		shaderModuleCreateInfo := vk.ShaderModuleCreateInfo{
			SType:    vk.StructureTypeShaderModuleCreateInfo,
			CodeSize: uint(len(shaderCode)),
			PCode:    sliceUint32(shaderCode),
		}
		if result := vk.CreateShaderModule(deviceHandle, &shaderModuleCreateInfo, nil, &vertShaderModule); result != vk.Success {
			log.Err(vk.Error(result), "create vertex shader")
			return
		}
	}
	defer vk.DestroyShaderModule(deviceHandle, vertShaderModule, nil)
	{
		shaderCode, err := ioutil.ReadFile("tri.frag.spv")
		if err != nil {
			log.Err(err, "read frag code")
			return
		}

		shaderModuleCreateInfo := vk.ShaderModuleCreateInfo{
			SType:    vk.StructureTypeShaderModuleCreateInfo,
			CodeSize: uint(len(shaderCode)),
			PCode:    sliceUint32(shaderCode),
		}
		if result := vk.CreateShaderModule(deviceHandle, &shaderModuleCreateInfo, nil, &fragShaderModule); result != vk.Success {
			log.Err(vk.Error(result), "create frag shader")
			return
		}
	}
	defer vk.DestroyShaderModule(deviceHandle, fragShaderModule, nil)

	// Shader stages
	// PName must be "main"???
	// Oh.. it's the name of the entry function in the shader...
	shaderStageCreateInfos := []vk.PipelineShaderStageCreateInfo{
		{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageVertexBit,
			Module: vertShaderModule,
			PName:  vkString("main"),
		},
		{
			SType:  vk.StructureTypePipelineShaderStageCreateInfo,
			Stage:  vk.ShaderStageFragmentBit,
			Module: fragShaderModule,
			PName:  vkString("main"),
		},
	}

	// Vertex Input
	vertexInputStateCreateInfo := vk.PipelineVertexInputStateCreateInfo{
		SType: vk.StructureTypePipelineVertexInputStateCreateInfo,
	}

	// Input assembly
	inputAssemblyStateCreateInfo := vk.PipelineInputAssemblyStateCreateInfo{
		SType:                  vk.StructureTypePipelineInputAssemblyStateCreateInfo,
		Topology:               vk.PrimitiveTopologyTriangleList,
		PrimitiveRestartEnable: vk.False,
	}

	// Viewport
	viewport := vk.Viewport{
		X:        0.0,
		Y:        0.0,
		Width:    float32(framebufferWidth),
		Height:   float32(framebufferHeight),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}
	scissor := vk.Rect2D{
		Offset: vk.Offset2D{
			X: 0,
			Y: 0,
		},
		Extent: vk.Extent2D{
			Width:  framebufferWidth,
			Height: framebufferHeight,
		},
	}
	viewportStateCreateInfo := vk.PipelineViewportStateCreateInfo{
		SType:         vk.StructureTypePipelineViewportStateCreateInfo,
		ViewportCount: 1,
		PViewports: []vk.Viewport{
			viewport,
		},
		ScissorCount: 1,
		PScissors: []vk.Rect2D{
			scissor,
		},
	}

	// Raster state
	rasterStateCreateInfo := vk.PipelineRasterizationStateCreateInfo{
		SType:                   vk.StructureTypePipelineRasterizationStateCreateInfo,
		DepthClampEnable:        vk.False,
		RasterizerDiscardEnable: vk.False,
		PolygonMode:             vk.PolygonModeFill,
		CullMode:                vk.CullModeFlags(vk.CullModeBackBit),
		FrontFace:               vk.FrontFaceCounterClockwise,
		DepthBiasEnable:         vk.False,
		DepthBiasConstantFactor: 0.0,
		DepthBiasClamp:          0.0,
		DepthBiasSlopeFactor:    0.0,
		LineWidth:               1.0,
	}

	// Multisample state
	multisampleStateCreateInfo := vk.PipelineMultisampleStateCreateInfo{
		SType:                 vk.StructureTypePipelineMultisampleStateCreateInfo,
		RasterizationSamples:  vk.SampleCount1Bit,
		SampleShadingEnable:   vk.False,
		MinSampleShading:      1.0,
		AlphaToCoverageEnable: vk.False,
		AlphaToOneEnable:      vk.False,
	}

	// Blending state
	colorBlendAttachmentState := vk.PipelineColorBlendAttachmentState{
		BlendEnable:         vk.False,
		SrcColorBlendFactor: vk.BlendFactorOne,
		DstColorBlendFactor: vk.BlendFactorZero,
		ColorBlendOp:        vk.BlendOpAdd,
		SrcAlphaBlendFactor: vk.BlendFactorOne,
		DstAlphaBlendFactor: vk.BlendFactorZero,
		AlphaBlendOp:        vk.BlendOpAdd,
		ColorWriteMask:      vk.ColorComponentFlags(vk.ColorComponentRBit | vk.ColorComponentGBit | vk.ColorComponentBBit | vk.ColorComponentABit),
	}
	colorBlendStateCreateInfo := vk.PipelineColorBlendStateCreateInfo{
		SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
		LogicOpEnable:   vk.False,
		LogicOp:         vk.LogicOpCopy,
		AttachmentCount: 1,
		PAttachments: []vk.PipelineColorBlendAttachmentState{
			colorBlendAttachmentState,
		},
		BlendConstants: [4]float32{0.0, 0.0, 0.0, 0.0},
	}

	// Dynamic state
	dynamicStateCreateInfo := vk.PipelineDynamicStateCreateInfo{
		SType: vk.StructureTypePipelineDynamicStateCreateInfo,
	}

	// Pipeline layout
	layoutCreateInfo := vk.PipelineLayoutCreateInfo{
		SType: vk.StructureTypePipelineLayoutCreateInfo,
	}
	var pipelineLayout vk.PipelineLayout
	if result := vk.CreatePipelineLayout(deviceHandle, &layoutCreateInfo, nil, &pipelineLayout); result != vk.Success {
		log.Err(vk.Error(result), "create pipeline layout")
		return
	}
	defer vk.DestroyPipelineLayout(deviceHandle, pipelineLayout, nil)

	// Graphics pipeline
	pipelineCreateInfo := []vk.GraphicsPipelineCreateInfo{
		{
			SType:               vk.StructureTypeGraphicsPipelineCreateInfo,
			StageCount:          uint32(len(shaderStageCreateInfos)),
			PStages:             shaderStageCreateInfos,
			PVertexInputState:   &vertexInputStateCreateInfo,
			PDynamicState:       &dynamicStateCreateInfo,
			PInputAssemblyState: &inputAssemblyStateCreateInfo,
			PViewportState:      &viewportStateCreateInfo,
			PRasterizationState: &rasterStateCreateInfo,
			PMultisampleState:   &multisampleStateCreateInfo,
			PColorBlendState:    &colorBlendStateCreateInfo,
			Layout:              pipelineLayout,
			RenderPass:          renderPass,
		},
	}
	graphicsPipeline := make([]vk.Pipeline, 1)
	if result := vk.CreateGraphicsPipelines(deviceHandle, nil, 1, pipelineCreateInfo, nil, graphicsPipeline); result != vk.Success {
		log.Err(vk.Error(result), "create pipeline layout")
		return
	}
	defer vk.DestroyPipeline(deviceHandle, graphicsPipeline[0], nil)

	// Set up Command buffers
	// Command pool
	var graphicsQueueCmdPool vk.CommandPool
	graphicsCmdPoolCreateInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: uint32(device.GraphicsIndex),
	}
	if result := vk.CreateCommandPool(deviceHandle, &graphicsCmdPoolCreateInfo, nil, &graphicsQueueCmdPool); result != vk.Success {
		log.Err(vk.Error(result), "create graphics command pool")
		return
	}
	defer vk.DestroyCommandPool(deviceHandle, graphicsQueueCmdPool, nil)

	// Set up Command buffers
	graphicsQueueCmdBuffers := make([]vk.CommandBuffer, imageCount)
	graphicsCmdBufferAllocateInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        graphicsQueueCmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: imageCount,
	}
	if result := vk.AllocateCommandBuffers(deviceHandle, &graphicsCmdBufferAllocateInfo, graphicsQueueCmdBuffers); result != vk.Success {
		log.Err(vk.Error(result), "allocate graphics command buffers")
		return
	}
	defer vk.FreeCommandBuffers(deviceHandle, graphicsQueueCmdPool, imageCount, graphicsQueueCmdBuffers)

	// Record the buffers
	graphicsCmdBufferBeginInfo := vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit),
	}
	graphicsSubresourceRange := vk.ImageSubresourceRange{
		AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
		LevelCount: 1,
		LayerCount: 1,
	}
	for i := range graphicsQueueCmdBuffers {
		vk.BeginCommandBuffer(graphicsQueueCmdBuffers[i], &graphicsCmdBufferBeginInfo)

		barrierFromPresentToDraw := vk.ImageMemoryBarrier{
			SType:               vk.StructureTypeImageMemoryBarrier,
			SrcAccessMask:       vk.AccessFlags(vk.AccessMemoryReadBit),
			DstAccessMask:       vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
			OldLayout:           vk.ImageLayoutPresentSrc,
			NewLayout:           vk.ImageLayoutPresentSrc,
			SrcQueueFamilyIndex: uint32(device.PresentIndex),
			DstQueueFamilyIndex: uint32(device.GraphicsIndex),
			Image:               swapChainImages[i],
			SubresourceRange:    graphicsSubresourceRange,
		}
		vk.CmdPipelineBarrier(graphicsQueueCmdBuffers[i], vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit), vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit), 0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{barrierFromPresentToDraw})

		renderPassBeginInfo := vk.RenderPassBeginInfo{
			SType:       vk.StructureTypeRenderPassBeginInfo,
			RenderPass:  renderPass,
			Framebuffer: framebuffers[i],
			RenderArea: vk.Rect2D{
				Offset: vk.Offset2D{
					X: 0,
					Y: 0,
				},
				Extent: vk.Extent2D{
					Width:  framebufferWidth,
					Height: framebufferHeight,
				},
			},
			ClearValueCount: 1,
			PClearValues: []vk.ClearValue{
				vk.NewClearValue([]float32{1.0, 0.8, 0.4, 0.0}),
			},
		}
		vk.CmdBeginRenderPass(graphicsQueueCmdBuffers[i], &renderPassBeginInfo, vk.SubpassContentsInline)
		vk.CmdBindPipeline(graphicsQueueCmdBuffers[i], vk.PipelineBindPointGraphics, graphicsPipeline[0])
		vk.CmdDraw(graphicsQueueCmdBuffers[i], 3, 1, 0, 0)
		vk.CmdEndRenderPass(graphicsQueueCmdBuffers[i])

		barrierFromDrawToPresent := vk.ImageMemoryBarrier{
			SType:               vk.StructureTypeImageMemoryBarrier,
			SrcAccessMask:       vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
			DstAccessMask:       vk.AccessFlags(vk.AccessMemoryReadBit),
			OldLayout:           vk.ImageLayoutPresentSrc,
			NewLayout:           vk.ImageLayoutPresentSrc,
			SrcQueueFamilyIndex: uint32(device.GraphicsIndex),
			DstQueueFamilyIndex: uint32(device.PresentIndex),
			Image:               swapChainImages[i],
			SubresourceRange:    graphicsSubresourceRange,
		}
		vk.CmdPipelineBarrier(graphicsQueueCmdBuffers[i], vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit), vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit), 0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{barrierFromDrawToPresent})

		if result := vk.EndCommandBuffer(graphicsQueueCmdBuffers[i]); result != vk.Success {
			log.Err(vk.Error(result), "record graphics command buffer")
			return
		}
	}
	// -Set up render pass

	fmt.Println("Drawing")
	for !framework.ShouldClose() {
		var imageIndex uint32
		result := vk.AcquireNextImage(deviceHandle, swapchain, vk.MaxUint64, imageAvailableSemaphore.Handle(), vk.NullFence, &imageIndex)
		switch result {
		case vk.Success:
			fallthrough
		case vk.Suboptimal:
		case vk.ErrorOutOfDate:
			fmt.Println("aquire outdate")
			log.Log("aquire outdate")
		default:
			log.Err(vk.Error(result), "aquire image")
			return
		}

		submitInfo := vk.SubmitInfo{
			SType:              vk.StructureTypeSubmitInfo,
			WaitSemaphoreCount: 1,
			PWaitSemaphores: []vk.Semaphore{
				imageAvailableSemaphore.Handle(),
			},
			PWaitDstStageMask: []vk.PipelineStageFlags{
				vk.PipelineStageFlags(vk.PipelineStageTransferBit),
			},
			CommandBufferCount: 1,
			PCommandBuffers: []vk.CommandBuffer{
				graphicsQueueCmdBuffers[imageIndex],
			},
			SignalSemaphoreCount: 1,
			PSignalSemaphores: []vk.Semaphore{
				renderingFinishedSemaphore.Handle(),
			},
		}
		if result := vk.QueueSubmit(graphicsQueue, 1, []vk.SubmitInfo{
			submitInfo,
		}, vk.NullFence); result != vk.Success {
			log.Err(vk.Error(result), "queue submit")
			return
		}

		presentInfo := vk.PresentInfo{
			SType:              vk.StructureTypePresentInfo,
			WaitSemaphoreCount: 1,
			PWaitSemaphores: []vk.Semaphore{
				renderingFinishedSemaphore.Handle(),
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
			log.Log("present outdate")
		default:
			log.Err(vk.Error(result), "image present")
			return
		}

		glfw.PollEvents()
	}
	framework.BackendDevice().WaitIdle()

	log.Log("fin")
}
