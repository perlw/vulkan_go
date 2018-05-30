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
	window, err := glfw.CreateWindow(640, 480, "Test test", nil, nil)
	if err != nil {
		panic(err.Error())
	}
	defer window.Destroy()

	exts := vk.GetRequiredInstanceExtensions()
	//exts = append(exts, "VK_EXT_debug_report")

	instanceInfo := vk.InstanceCreateInfo{
		SType: vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: &vk.ApplicationInfo{
			SType:              vk.StructureTypeApplicationInfo,
			PApplicationName:   "Abyssal_Drifter\x00",
			PEngineName:        "HARLE\x00",
			ApiVersion:         vk.MakeVersion(1, 0, 0),
			ApplicationVersion: vk.MakeVersion(1, 0, 0),
		},
		EnabledLayerCount:       0,
		PpEnabledLayerNames:     nil,
		EnabledExtensionCount:   uint32(len(exts)),
		PpEnabledExtensionNames: exts,
	}

	var instance vk.Instance
	if result := vk.CreateInstance(&instanceInfo, nil, &instance); result != vk.Success {
		fmt.Println("err:", result)
		return
	}
	defer vk.DestroyInstance(instance, nil)

	vk.InitInstance(instance)

	var gpuCount uint32
	if result := vk.EnumeratePhysicalDevices(instance, &gpuCount, nil); result != vk.Success {
		fmt.Println("err:", result)
		return
	}
	if gpuCount == 0 {
		fmt.Println("err: no gpus")
		return
	}
	fmt.Println("GPUS:", gpuCount)
	gpus := make([]vk.PhysicalDevice, gpuCount)
	if result := vk.EnumeratePhysicalDevices(instance, &gpuCount, gpus); result != vk.Success {
		fmt.Println("err:", result)
		return
	}
	gpu := gpus[0]

	var gpuProperties vk.PhysicalDeviceProperties
	var memoryProperties vk.PhysicalDeviceMemoryProperties
	vk.GetPhysicalDeviceProperties(gpu, &gpuProperties)
	gpuProperties.Deref()
	vk.GetPhysicalDeviceMemoryProperties(gpu, &memoryProperties)
	memoryProperties.Deref()

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

	/*for !window.ShouldClose() {
		glfw.PollEvents()
	}*/
}
