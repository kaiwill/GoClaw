package peripherals

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type PeripheralType string

const (
	PeripheralTypeArduino PeripheralType = "arduino"
	PeripheralTypeRPi     PeripheralType = "rpi"
	PeripheralTypeSTM32   PeripheralType = "stm32"
	PeripheralTypeESP32   PeripheralType = "esp32"
)

type Peripheral interface {
	Name() string
	Type() PeripheralType
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool
	Execute(ctx context.Context, command string, args []byte) ([]byte, error)
}

type Registry struct {
	mu          sync.RWMutex
	peripherals map[string]Peripheral
}

func NewRegistry() *Registry {
	return &Registry{
		peripherals: make(map[string]Peripheral),
	}
}

func (r *Registry) Register(name string, p Peripheral) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peripherals[name] = p
}

func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peripherals, name)
}

func (r *Registry) Get(name string) (Peripheral, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.peripherals[name]
	return p, ok
}

func (r *Registry) List() []Peripheral {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Peripheral, 0, len(r.peripherals))
	for _, p := range r.peripherals {
		list = append(list, p)
	}
	return list
}

func (r *Registry) ConnectAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, p := range r.peripherals {
		if !p.IsConnected() {
			if err := p.Connect(ctx); err != nil {
				return fmt.Errorf("failed to connect %s: %w", name, err)
			}
		}
	}
	return nil
}

func (r *Registry) DisconnectAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, p := range r.peripherals {
		if p.IsConnected() {
			if err := p.Disconnect(ctx); err != nil {
				return fmt.Errorf("failed to disconnect %s: %w", name, err)
			}
		}
	}
	return nil
}

type BasePeripheral struct {
	name       string
	periphType PeripheralType
	connected  bool
	mu         sync.RWMutex
}

func (p *BasePeripheral) Name() string {
	return p.name
}

func (p *BasePeripheral) Type() PeripheralType {
	return p.periphType
}

func (p *BasePeripheral) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connected
}

func (p *BasePeripheral) setConnected(connected bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.connected = connected
}

type ArduinoPeripheral struct {
	BasePeripheral
	port string
	baud int
}

func NewArduinoPeripheral(name, port string, baud int) *ArduinoPeripheral {
	return &ArduinoPeripheral{
		BasePeripheral: BasePeripheral{
			name:       name,
			periphType: PeripheralTypeArduino,
		},
		port: port,
		baud: baud,
	}
}

func (a *ArduinoPeripheral) Connect(ctx context.Context) error {
	if a.IsConnected() {
		return nil
	}
	// Simplified - would use serial port library
	a.setConnected(true)
	return nil
}

func (a *ArduinoPeripheral) Disconnect(ctx context.Context) error {
	if !a.IsConnected() {
		return nil
	}
	a.setConnected(false)
	return nil
}

func (a *ArduinoPeripheral) Execute(ctx context.Context, command string, args []byte) ([]byte, error) {
	if !a.IsConnected() {
		return nil, fmt.Errorf("arduino not connected")
	}
	// Simplified - would send to serial port
	return []byte("OK"), nil
}

type RPiPeripheral struct {
	BasePeripheral
	interfaceType string
}

func NewRPiPeripheral(name, interfaceType string) *RPiPeripheral {
	return &RPiPeripheral{
		BasePeripheral: BasePeripheral{
			name:       name,
			periphType: PeripheralTypeRPi,
		},
		interfaceType: interfaceType,
	}
}

func (r *RPiPeripheral) Connect(ctx context.Context) error {
	if r.IsConnected() {
		return nil
	}
	// Simplified - would use periph.io or similar
	r.setConnected(true)
	return nil
}

func (r *RPiPeripheral) Disconnect(ctx context.Context) error {
	if !r.IsConnected() {
		return nil
	}
	r.setConnected(false)
	return nil
}

func (r *RPiPeripheral) Execute(ctx context.Context, command string, args []byte) ([]byte, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("rpi not connected")
	}
	// Simplified - would execute GPIO/I2C/SPI commands
	return []byte("OK"), nil
}

type STM32Peripheral struct {
	BasePeripheral
	devicePath string
}

func NewSTM32Peripheral(name, devicePath string) *STM32Peripheral {
	return &STM32Peripheral{
		BasePeripheral: BasePeripheral{
			name:       name,
			periphType: PeripheralTypeSTM32,
		},
		devicePath: devicePath,
	}
}

func (s *STM32Peripheral) Connect(ctx context.Context) error {
	if s.IsConnected() {
		return nil
	}
	// Simplified - would use probe-rs or openocd
	s.setConnected(true)
	return nil
}

func (s *STM32Peripheral) Disconnect(ctx context.Context) error {
	if !s.IsConnected() {
		return nil
	}
	s.setConnected(false)
	return nil
}

func (s *STM32Peripheral) Execute(ctx context.Context, command string, args []byte) ([]byte, error) {
	if !s.IsConnected() {
		return nil, fmt.Errorf("stm32 not connected")
	}
	// Simplified - would flash/debug via probe
	return []byte("OK"), nil
}

func (s *STM32Peripheral) Flash(ctx context.Context, firmware []byte) error {
	if !s.IsConnected() {
		return fmt.Errorf("stm32 not connected")
	}
	// Simplified - would flash firmware
	return nil
}

type ESP32Peripheral struct {
	BasePeripheral
	port string
	baud int
}

func NewESP32Peripheral(name, port string, baud int) *ESP32Peripheral {
	return &ESP32Peripheral{
		BasePeripheral: BasePeripheral{
			name:       name,
			periphType: PeripheralTypeESP32,
		},
		port: port,
		baud: baud,
	}
}

func (e *ESP32Peripheral) Connect(ctx context.Context) error {
	if e.IsConnected() {
		return nil
	}
	e.setConnected(true)
	return nil
}

func (e *ESP32Peripheral) Disconnect(ctx context.Context) error {
	if !e.IsConnected() {
		return nil
	}
	e.setConnected(false)
	return nil
}

func (e *ESP32Peripheral) Execute(ctx context.Context, command string, args []byte) ([]byte, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("esp32 not connected")
	}
	return []byte("OK"), nil
}

func init() {
	_ = time.Second
}
