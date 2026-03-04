package hardware

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type DeviceType string

const (
	DeviceTypeUSB     DeviceType = "usb"
	DeviceTypeSerial  DeviceType = "serial"
	DeviceTypeArduino DeviceType = "arduino"
	DeviceTypeSTM32   DeviceType = "stm32"
	DeviceTypeRPi     DeviceType = "rpi"
	DeviceTypeESP32   DeviceType = "esp32"
	DeviceTypeUnknown DeviceType = "unknown"
)

type Device struct {
	ID          string
	Name        string
	Type        DeviceType
	VendorID    string
	ProductID   string
	SerialNum   string
	Path        string
	IsConnected bool
	LastSeen    time.Time
}

type DiscoveryResult struct {
	Devices []Device
	Errors  []error
}

type DiscoveryFunc func(ctx context.Context) ([]Device, error)

type Manager struct {
	mu           sync.RWMutex
	devices      map[string]*Device
	discoveryFns map[DeviceType]DiscoveryFunc
	interval     time.Duration
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewManager(interval time.Duration) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		devices:      make(map[string]*Device),
		discoveryFns: make(map[DeviceType]DiscoveryFunc),
		interval:     interval,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (m *Manager) RegisterDiscovery(deviceType DeviceType, fn DiscoveryFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.discoveryFns[deviceType] = fn
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("hardware discovery already running")
	}
	m.running = true
	m.mu.Unlock()

	go m.run()
	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		m.cancel()
		m.running = false
	}
}

func (m *Manager) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.discover()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.discover()
		}
	}
}

func (m *Manager) discover() {
	m.mu.RLock()
	fns := make(map[DeviceType]DiscoveryFunc)
	for k, v := range m.discoveryFns {
		fns[k] = v
	}
	m.mu.RUnlock()

	result := DiscoveryResult{
		Devices: []Device{},
		Errors:  []error{},
	}

	for deviceType, fn := range fns {
		devices, err := fn(m.ctx)
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
		result.Devices = append(result.Devices, devices...)
		_ = deviceType
	}

	m.mu.Lock()
	for _, dev := range result.Devices {
		m.devices[dev.ID] = &dev
	}
	m.mu.Unlock()
}

func (m *Manager) GetDevices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]Device, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

func (m *Manager) GetDevice(id string) (*Device, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.devices[id]
	return d, ok
}

func (m *Manager) Refresh(ctx context.Context) {
	m.discover()
}

func discoverUSB(ctx context.Context) ([]Device, error) {
	var devices []Device

	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("system_profiler", "SPUSBDataType").Output()
		if err != nil {
			return devices, nil
		}
		devices = parseUSBOutput(string(out))

	case "linux":
		out, err := exec.Command("lsusb").Output()
		if err != nil {
			return devices, nil
		}
		devices = parseLSUSB(string(out))

	case "windows":
		out, err := exec.Command("wmic", "path", "Win32_USBControllerDevice", "get", "DeviceID").Output()
		if err != nil {
			return devices, nil
		}
		devices = parseWMIC(string(out))
	}

	return devices, nil
}

func discoverSerial(ctx context.Context) ([]Device, error) {
	var devices []Device

	switch runtime.GOOS {
	case "darwin":
		entries, err := os.ReadDir("/dev")
		if err != nil {
			return devices, nil
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "cu.") {
				devices = append(devices, Device{
					ID:   entry.Name(),
					Name: entry.Name(),
					Type: DeviceTypeSerial,
					Path: "/dev/" + entry.Name(),
				})
			}
		}

	case "linux":
		entries, err := os.ReadDir("/dev")
		if err != nil {
			return devices, nil
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "ttyUSB") || strings.HasPrefix(entry.Name(), "ttyACM") {
				devices = append(devices, Device{
					ID:   entry.Name(),
					Name: entry.Name(),
					Type: DeviceTypeSerial,
					Path: "/dev/" + entry.Name(),
				})
			}
		}
	}

	return devices, nil
}

func parseUSBOutput(output string) []Device {
	var devices []Device
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Arduino") {
			devices = append(devices, Device{
				Name: strings.TrimSpace(line),
				Type: DeviceTypeArduino,
			})
		} else if strings.Contains(line, "STMicroelectronics") {
			devices = append(devices, Device{
				Name: strings.TrimSpace(line),
				Type: DeviceTypeSTM32,
			})
		}
	}
	return devices
}

func parseLSUSB(output string) []Device {
	var devices []Device
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Arduino") || strings.Contains(line, "2341") || strings.Contains(line, "2a03") {
			devices = append(devices, Device{
				Name: strings.TrimSpace(line),
				Type: DeviceTypeArduino,
			})
		} else if strings.Contains(line, "STMicro") || strings.Contains(line, "0483") {
			devices = append(devices, Device{
				Name: strings.TrimSpace(line),
				Type: DeviceTypeSTM32,
			})
		}
	}
	return devices
}

func parseWMIC(output string) []Device {
	var devices []Device
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "USB") && strings.Contains(line, "VID_") {
			devices = append(devices, Device{
				Name: line,
				Type: DeviceTypeUSB,
			})
		}
	}
	return devices
}

func detectDeviceType(path string) DeviceType {
	if strings.Contains(path, "Arduino") {
		return DeviceTypeArduino
	}
	if strings.Contains(path, "STM") || strings.Contains(path, "STMicro") {
		return DeviceTypeSTM32
	}
	if strings.Contains(path, "ESP") {
		return DeviceTypeESP32
	}
	return DeviceTypeUnknown
}

func init() {
	_ = detectDeviceType
}
