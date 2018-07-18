package pompeii

import (
	"github.com/pkg/errors"
	vk "github.com/vulkan-go/vulkan"
)

type Semaphore struct {
	logicalDevice vk.Device
	semaphore     vk.Semaphore
}

func NewSemaphore(d *Device) (*Semaphore, error) {
	s := Semaphore{
		logicalDevice: d.Handle(),
	}

	semaphoreCreateInfo := vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	if result := vk.CreateSemaphore(d.Handle(), &semaphoreCreateInfo, nil, &s.semaphore); result != vk.Success {
		return nil, errors.Wrap(vk.Error(result), "create semaphore")
	}

	return &s, nil
}

func (s *Semaphore) Destroy() {
	if s.semaphore != vk.NullSemaphore {
		vk.DestroySemaphore(s.logicalDevice, s.semaphore, nil)
	}
}

func (s *Semaphore) Handle() vk.Semaphore {
	return s.semaphore
}
