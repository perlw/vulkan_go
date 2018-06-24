package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/vulkan-go/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

func init() {
	runtime.LockOSThread()
}

type Logger struct {
	log   *log.Logger
	warn  *log.Logger
	err   *log.Logger
	trace *log.Logger
}

func NewLogger(prefix string) Logger {
	return Logger{
		log:   log.New(os.Stdout, "["+prefix+"] ", log.Ldate|log.Ltime),
		warn:  log.New(os.Stderr, "["+prefix+" WARN] ", log.Ldate|log.Ltime|log.Lshortfile),
		err:   log.New(os.Stderr, "["+prefix+" ERR] ", log.Ldate|log.Ltime|log.Llongfile),
		trace: log.New(os.Stderr, "["+prefix+" ERR] ", log.Ldate|log.Ltime|log.Llongfile),
	}
}

func (l Logger) Log(format string, a ...interface{}) {
	l.log.Printf(format, a...)
}
func (l Logger) Warn(format string, a ...interface{}) {
	l.warn.Printf(format, a...)
}
func (l Logger) Err(err error, format string, a ...interface{}) {
	if err != nil {
		l.err.Printf(format+","+err.Error(), a...)
	} else {
		l.err.Printf(format, a...)
	}
}
func (l Logger) Trace(format string, a ...interface{}) {
	l.trace.Printf(format, a...)
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

const EngineName = "MYR"

func inStringSlice(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
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

type myrInternal struct {
	instance vk.Instance
	dbg      vk.DebugReportCallback
}

type Myr struct {
	log Logger

	internal *myrInternal
}

func NewMyr(appName string) (*Myr, error) {
	myr := Myr{
		log:      NewLogger(EngineName),
		internal: &myrInternal{},
	}

	if err := vk.Init(); err != nil {
		return nil, errors.Wrap(err, "could not initialize vulkan")
	}

	debug := false
	layers := []string{}
	{
		available, err := getAvailableInstanceLayers()
		if err != nil {
			return nil, errors.Wrap(err, "could not get layers")
		}
		if inStringSlice(available, "VK_LAYER_LUNARG_standard_validation") {
			layers = append(layers, vkString("VK_LAYER_LUNARG_standard_validation"))
		}
	}

	exts := vk.GetRequiredInstanceExtensions()
	{
		available, err := getAvailableInstanceExtensions()
		if err != nil {
			return nil, errors.Wrap(err, "could not get instance extensions")
		}
		if inStringSlice(available, "VK_EXT_debug_report") {
			debug = true
			exts = append(exts, vkString("VK_EXT_debug_report"))
		}
	}

	instanceInfo := vk.InstanceCreateInfo{
		SType: vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: &vk.ApplicationInfo{
			SType:              vk.StructureTypeApplicationInfo,
			PApplicationName:   vkString(appName),
			ApplicationVersion: vk.MakeVersion(1, 0, 0),
			PEngineName:        vkString(EngineName),
			EngineVersion:      vk.MakeVersion(0, 0, 1),
			ApiVersion:         vk.ApiVersion10,
		},
		EnabledLayerCount:       uint32(len(layers)),
		PpEnabledLayerNames:     layers,
		EnabledExtensionCount:   uint32(len(exts)),
		PpEnabledExtensionNames: exts,
	}

	if result := vk.CreateInstance(&instanceInfo, nil, &myr.internal.instance); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "could not create instance")
	}

	myr.log.Log("instance created;\n\tlayers: %v\n\texts: %v\n", layers, exts)

	vk.InitInstance(myr.internal.instance)

	// +Debug
	if debug {
		debugCreateInfo := vk.DebugReportCallbackCreateInfo{
			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit),
			PfnCallback: myr.debugReportCallback,
		}
		if result := vk.CreateDebugReportCallback(myr.internal.instance, &debugCreateInfo, nil, &myr.internal.dbg); result != vk.Success {
			myr.log.Err(vk.Error(result), "creating debug report")
		}
	}
	// -Debug

	return &myr, nil
}

func (m *Myr) Destroy() {
	if m.internal.dbg != vk.NullDebugReportCallback {
		vk.DestroyDebugReportCallback(m.internal.instance, m.internal.dbg, nil)
	}
	vk.DestroyInstance(m.internal.instance, nil)
}

func (m *Myr) debugReportCallback(flags vk.DebugReportFlags, objectType vk.DebugReportObjectType,
	object uint64, location uint, messageCode int32, pLayerPrefix string,
	pMessage string, pUserData unsafe.Pointer) vk.Bool32 {
	switch {
	case flags&vk.DebugReportFlags(vk.DebugReportErrorBit) != 0:
		m.log.Log("[VK DBG ERR %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	case flags&vk.DebugReportFlags(vk.DebugReportWarningBit) != 0:
		m.log.Log("[VK DBG WARN %d] %s on layer %s", messageCode, pMessage, pLayerPrefix)
	default:
		m.log.Log("[VK DBG UNK] unknown debug message %d (layer %s)", messageCode, pLayerPrefix)
	}
	return vk.Bool32(vk.False)
}

func (m *Myr) EnumerateGPUs() ([]GPU, error) {
	var gpuCount uint32
	if result := vk.EnumeratePhysicalDevices(m.internal.instance, &gpuCount, nil); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "could not count gpus")
	}
	if gpuCount == 0 {
		return nil, errors.New("no valid gpus")
	}
	vkGPUs := make([]vk.PhysicalDevice, gpuCount)
	if result := vk.EnumeratePhysicalDevices(m.internal.instance, &gpuCount, vkGPUs); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "could not enumerate gpus")
	}

	gpus := make([]GPU, gpuCount)
	for t, gpu := range vkGPUs {
		gpus[t] = createGPU(gpu)
	}

	return gpus, nil
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

type gpuInternal struct {
	gpu      vk.PhysicalDevice
	props    vk.PhysicalDeviceProperties
	memProps vk.PhysicalDeviceMemoryProperties
	features vk.PhysicalDeviceFeatures
}

type GPU struct {
	Name string
	Type GPUType

	internal *gpuInternal
}

func createGPU(vkGPU vk.PhysicalDevice) GPU {
	gpu := GPU{
		internal: &gpuInternal{
			gpu: vkGPU,
		},
	}

	vk.GetPhysicalDeviceProperties(gpu.internal.gpu, &gpu.internal.props)
	gpu.internal.props.Deref()
	gpu.internal.props.Limits.Deref()
	gpu.internal.props.SparseProperties.Deref()

	vk.GetPhysicalDeviceMemoryProperties(gpu.internal.gpu, &gpu.internal.memProps)
	gpu.internal.memProps.Deref()

	vk.GetPhysicalDeviceFeatures(gpu.internal.gpu, &gpu.internal.features)
	gpu.internal.features.Deref()

	gpu.Name = string(gpu.internal.props.DeviceName[:])
	gpu.Type = GPUType(gpu.internal.props.DeviceType)

	return gpu
}

func (g GPU) Debug() string {
	buffer := bytes.Buffer{}

	buffer.WriteString(fmt.Sprintln("Device Name:", g.Name))
	buffer.WriteString(fmt.Sprintln("Device Type:", g.Type))
	buffer.WriteString("## Backend\n")
	buffer.WriteString(fmt.Sprintf("Vulkan v%d.%d.%d\n",
		(g.internal.props.ApiVersion>>22)&0x3ff,
		(g.internal.props.ApiVersion>>12)&0x3ff,
		g.internal.props.ApiVersion&0xfff,
	))
	buffer.WriteString(fmt.Sprintf("Driver v%d.%d.%d\n",
		(g.internal.props.DriverVersion>>22)&0x3ff,
		(g.internal.props.DriverVersion>>12)&0x3ff,
		g.internal.props.DriverVersion&0xfff,
	))
	buffer.WriteString(fmt.Sprintln("Max Image Dimension:", g.internal.props.Limits.MaxImageDimension2D))
	buffer.WriteString(fmt.Sprintln("Max Viewports:", g.internal.props.Limits.MaxViewports))
	buffer.WriteString(fmt.Sprintln("Max Viewport Dimensions:", g.internal.props.Limits.MaxViewportDimensions[0], g.internal.props.Limits.MaxViewportDimensions[1]))

	return buffer.String()
}

func (g GPU) Match(resWidth, resHeight uint32) bool {
	return (g.internal.props.Limits.MaxViewportDimensions[0] >= resWidth && g.internal.props.Limits.MaxViewportDimensions[1] >= resHeight)
}

const APP_NAME = "Abyssal Drifter"
const RES_WIDTH = 640
const RES_HEIGHT = 480

func main() {
	log := NewLogger(APP_NAME)

	glfw.Init()
	defer glfw.Terminate()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.False)
	window, err := glfw.CreateWindow(RES_WIDTH, RES_HEIGHT, APP_NAME, nil, nil)
	if err != nil {
		panic(err.Error())
	}
	defer window.Destroy()

	// +Set up Vulkan
	myr, err := NewMyr(APP_NAME)
	if err != nil {
		panic(err.Error())
		return
	}
	defer myr.Destroy()

	gpus, err := myr.EnumerateGPUs()
	if err != nil {
		log.Err(err, "enumerating GPUs")
		return
	}
	var chosenGPU *GPU
	for t, gpu := range gpus {
		log.Log("# GPU %d\n%s", t, gpu.Debug())
		log.Log(gpu.Debug())
		if gpu.Match(RES_WIDTH, RES_HEIGHT) {
			chosenGPU = &gpus[t]
		}
	}
	if chosenGPU == nil {
		log.Err(nil, "no matching GPU")
		return
	}
	gpuHandle := chosenGPU.internal.gpu
	log.Log("Picked: %s\n", chosenGPU.Name)

	// Surface
	var surface vk.Surface
	if result := vk.CreateWindowSurface(myr.internal.instance, window.GLFWWindow(), nil, &surface); result != vk.Success {
		log.Err(vk.Error(result), "create window surface")
		return
	}
	defer vk.DestroySurface(myr.internal.instance, surface, nil)

	// Check queue families
	var queueFamilyCount uint32
	vk.GetPhysicalDeviceQueueFamilyProperties(gpuHandle, &queueFamilyCount, nil)
	if queueFamilyCount == 0 {
		log.Err(nil, "no queue families")
		return
	}

	log.Log("Queue Families: %d", queueFamilyCount)
	queueFamilies := make([]vk.QueueFamilyProperties, queueFamilyCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(gpuHandle, &queueFamilyCount, queueFamilies)
	var graphicsFamilyIndex uint32
	var presentFamilyIndex uint32
	for i, family := range queueFamilies {
		family.Deref()

		var presentSupport vk.Bool32
		vk.GetPhysicalDeviceSurfaceSupport(gpuHandle, uint32(i), surface, &presentSupport)

		if family.QueueCount > 0 && family.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0 {
			graphicsFamilyIndex = uint32(i)
			if presentSupport > 0 {
				presentFamilyIndex = uint32(i)
			}
		}
		if family.QueueFlags&vk.QueueFlags(vk.QueueGraphicsBit) != 0 {
			log.Log("family: %d %s", i, "graphics")
		}
		if family.QueueFlags&vk.QueueFlags(vk.QueueComputeBit) != 0 {
			log.Log("family: %d %s", i, "compute")
		}
		if family.QueueFlags&vk.QueueFlags(vk.QueueTransferBit) != 0 {
			log.Log("family: %d %s", i, "transfer")
		}
	}
	log.Log("Graphics family index: %d", graphicsFamilyIndex)
	log.Log("Present family index: %d", presentFamilyIndex)

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
		PpEnabledExtensionNames: []string{vkString("VK_KHR_swapchain")},
	}
	var device vk.Device
	if result := vk.CreateDevice(gpuHandle, &deviceCreateInfo, nil, &device); result != vk.Success {
		fmt.Println("err:", "create device", result)
		return
	}
	defer (func() {
		vk.DeviceWaitIdle(device)
		vk.DestroyDevice(device, nil)
	})()
	// -Set up Vulkan

	// +Prepare rendering
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
		log.Err(vk.Error(result), "create semaphore, image")
		return
	}
	if result := vk.CreateSemaphore(device, &semaphoreCreateInfo, nil, &renderingFinishedSemaphore); result != vk.Success {
		log.Err(vk.Error(result), "create semaphore, rendering")
		return
	}
	defer vk.DestroySemaphore(device, imageAvailableSemaphore, nil)
	defer vk.DestroySemaphore(device, renderingFinishedSemaphore, nil)

	// Swap chain
	var surfaceCapabilities vk.SurfaceCapabilities
	if result := vk.GetPhysicalDeviceSurfaceCapabilities(gpuHandle, surface, &surfaceCapabilities); result != vk.Success {
		log.Err(vk.Error(result), "get surface caps")
		return
	}
	surfaceCapabilities.Deref()
	surfaceCapabilities.MinImageExtent.Deref()
	surfaceCapabilities.MaxImageExtent.Deref()

	log.Log("surface min: %dx%d", surfaceCapabilities.MinImageExtent.Width, surfaceCapabilities.MinImageExtent.Height)
	log.Log("surface max: %dx%d", surfaceCapabilities.MaxImageExtent.Width, surfaceCapabilities.MaxImageExtent.Height)

	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(gpuHandle, surface, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(gpuHandle, surface, &formatCount, formats)
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
			Width:  RES_WIDTH,
			Height: RES_HEIGHT,
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
		log.Err(vk.Error(result), "create swapchain")
		return
	}
	if oldSwapchain != vk.NullSwapchain {
		vk.DestroySwapchain(device, oldSwapchain, nil)
	}
	defer vk.DestroySwapchain(device, swapchain, nil)
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
	if result := vk.CreateRenderPass(device, &renderPassCreateInfo, nil, &renderPass); result != vk.Success {
		log.Err(vk.Error(result), "create render pass")
		return
	}
	defer vk.DestroyRenderPass(device, renderPass, nil)

	// Creating framebuffers
	var imageCount uint32
	if result := vk.GetSwapchainImages(device, swapchain, &imageCount, nil); result != vk.Success {
		log.Err(vk.Error(result), "get swapchain image count")
		return
	}
	log.Log("Swapchain image count: %d", imageCount)

	swapChainImages := make([]vk.Image, imageCount)
	if result := vk.GetSwapchainImages(device, swapchain, &imageCount, swapChainImages); result != vk.Success {
		log.Err(vk.Error(result), "get swapchain images")
		return
	}

	// TODO: Use single framebuffer, render to texture, then make swapchain copy from texture
	framebufferWidth := uint32(RES_WIDTH)
	framebufferHeight := uint32(RES_HEIGHT)
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
		if result := vk.CreateImageView(device, &imageViewCreateInfo, nil, &framebufferViews[i]); result != vk.Success {
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
		if result := vk.CreateFramebuffer(device, &framebufferCreateInfo, nil, &framebuffers[i]); result != vk.Success {
			log.Err(vk.Error(result), "create framebuffer")
			return
		}
	}

	defer (func() {
		for i := range swapChainImages {
			vk.DestroyImageView(device, framebufferViews[i], nil)
			vk.DestroyFramebuffer(device, framebuffers[i], nil)
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
		if result := vk.CreateShaderModule(device, &shaderModuleCreateInfo, nil, &vertShaderModule); result != vk.Success {
			log.Err(vk.Error(result), "create vertex shader")
			return
		}
	}
	defer vk.DestroyShaderModule(device, vertShaderModule, nil)
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
		if result := vk.CreateShaderModule(device, &shaderModuleCreateInfo, nil, &fragShaderModule); result != vk.Success {
			log.Err(vk.Error(result), "create frag shader")
			return
		}
	}
	defer vk.DestroyShaderModule(device, fragShaderModule, nil)

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
	if result := vk.CreatePipelineLayout(device, &layoutCreateInfo, nil, &pipelineLayout); result != vk.Success {
		log.Err(vk.Error(result), "create pipeline layout")
		return
	}
	defer vk.DestroyPipelineLayout(device, pipelineLayout, nil)

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
	if result := vk.CreateGraphicsPipelines(device, nil, 1, pipelineCreateInfo, nil, graphicsPipeline); result != vk.Success {
		log.Err(vk.Error(result), "create pipeline layout")
		return
	}
	defer vk.DestroyPipeline(device, graphicsPipeline[0], nil)

	// Set up Command buffers
	// Command pool
	var graphicsQueueCmdPool vk.CommandPool
	graphicsCmdPoolCreateInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: graphicsFamilyIndex,
	}
	if result := vk.CreateCommandPool(device, &graphicsCmdPoolCreateInfo, nil, &graphicsQueueCmdPool); result != vk.Success {
		log.Err(vk.Error(result), "create graphics command pool")
		return
	}
	defer vk.DestroyCommandPool(device, graphicsQueueCmdPool, nil)

	// Set up Command buffers
	graphicsQueueCmdBuffers := make([]vk.CommandBuffer, imageCount)
	graphicsCmdBufferAllocateInfo := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        graphicsQueueCmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: imageCount,
	}
	if result := vk.AllocateCommandBuffers(device, &graphicsCmdBufferAllocateInfo, graphicsQueueCmdBuffers); result != vk.Success {
		log.Err(vk.Error(result), "allocate graphics command buffers")
		return
	}
	defer vk.FreeCommandBuffers(device, graphicsQueueCmdPool, imageCount, graphicsQueueCmdBuffers)

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
			SrcQueueFamilyIndex: presentFamilyIndex,
			DstQueueFamilyIndex: graphicsFamilyIndex,
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
			SrcQueueFamilyIndex: graphicsFamilyIndex,
			DstQueueFamilyIndex: presentFamilyIndex,
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
	for !window.ShouldClose() {
		var imageIndex uint32
		result := vk.AcquireNextImage(device, swapchain, vk.MaxUint64, imageAvailableSemaphore, vk.NullFence, &imageIndex)
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
				imageAvailableSemaphore,
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
				renderingFinishedSemaphore,
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
			log.Log("present outdate")
		default:
			log.Err(vk.Error(result), "image present")
			return
		}

		glfw.PollEvents()
	}

	vk.DeviceWaitIdle(device)
	log.Log("fin")
}
