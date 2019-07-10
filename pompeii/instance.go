package pompeii

import (
	"fmt"
	"unsafe"

	"github.com/pkg/errors"
	vk "github.com/vulkan-go/vulkan"
)

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
			} else {
				fmt.Println("missing layer", name)
			}
		}
	}

	debug := false
	// activeExtensions := vk.GetRequiredInstanceExtensions()
	activeExtensions := make([]string, 0, 10)
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
			} else {
				fmt.Println("missing extension", name)
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
			ApiVersion:         vk.ApiVersion11,
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

func (i *Instance) Handle() vk.Instance {
	return i.instance
}
