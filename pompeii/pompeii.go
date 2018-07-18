package pompeii

import (
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
