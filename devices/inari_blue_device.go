package devices

import (
	"InariBlueNotifier/patterns"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	inariBlueI2CDeviceAddress = 0x45

	inariBlueRegisterGlobalControl     byte = 0x01
	inariBlueRegisterChannelEnable     byte = 0x30
	inariBlueRegisterChannelConfigBase byte = 0x31
	inariBlueRegisterChannelPWMBase    byte = 0x34
	inariBlueRegisterChannelTimingBase byte = 0x37

	inariBlueLEDChannelCount = 3

	inariBlueControllerEnabled  byte = 0x01
	inariBlueControllerDisabled byte = 0x00

	inariBlueChannelPatternMode byte = 0x9f

	inariBlueAllChannelsEnabled  byte = 0x07
	inariBlueAllChannelsDisabled byte = 0x00

	// The hardware timing formula is duration = timing unit * value².
	inariBlueTimingUnit = time.Duration(31.25 * float64(time.Millisecond))

	inariBlueControllerRestartDelay = 200 * time.Millisecond
)

var ErrClosed = errors.New("the Inari Blue device is closed")

// InariBlueDevice controls the Inari Blue's TOH LED controller.
type InariBlueDevice struct {
	mutex sync.Mutex

	// Connection and descriptor borrowed from Symbiosis.
	busConnection *dbus.Conn
	deviceFile    *os.File

	// Lifecycle state.
	closed   bool
	done     chan struct{}
	doneOnce sync.Once

	// Symbiosis availability monitor.
	monitorCancel context.CancelFunc
	monitorDone   chan struct{}
}

// NewInariBlueDevice borrows access to the TOH I2C bus from Symbiosis.
func NewInariBlueDevice(connection *dbus.Conn) (*InariBlueDevice, error) {
	return NewInariBlueDeviceWithContext(context.Background(), connection)
}

func NewInariBlueDeviceWithContext(ctx context.Context, connection *dbus.Conn) (*InariBlueDevice, error) {
	if !connection.SupportsUnixFDs() {
		return nil, errors.New("system D-Bus does not support Unix FD passing")
	}

	instance := &InariBlueDevice{
		busConnection: connection,
		done:          make(chan struct{}),
	}
	if err := instance.startMonitoring(); err != nil {
		return nil, fmt.Errorf("start Symbiosis monitoring: %w", err)
	}

	initialized := false
	defer func() {
		if !initialized {
			instance.stopMonitoring()
		}
	}()

	service := connection.Object(tohService, tohObject)
	var descriptor dbus.UnixFD
	if err := service.CallWithContext(ctx, tohInterface+".BorrowI2cDevAccessWithPower", 0, true).Store(&descriptor); err != nil {
		return nil, fmt.Errorf("borrow TOH I2C access from Symbiosis: %w", err)
	}

	deviceFile := os.NewFile(uintptr(descriptor), "symbiosis-toh-i2c")
	defer func() {
		if initialized {
			return
		}
		if deviceFile != nil {
			_ = deviceFile.Close()
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = service.CallWithContext(cleanupCtx, tohInterface+".ReturnI2cDevAccess", 0).Err
	}()
	if deviceFile == nil {
		return nil, errors.New("invalid I2C descriptor")
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, deviceFile.Fd(), tohI2CSlaveIOCTL, inariBlueI2CDeviceAddress)
	if errno != 0 {
		return nil, fmt.Errorf("select LED I2C address 0x%02x: %w", inariBlueI2CDeviceAddress, errno)
	}

	select {
	case <-instance.Done():
		return nil, errors.New("symbiosis became unavailable during LED initialization")
	default:
	}

	instance.deviceFile = deviceFile
	initialized = true

	return instance, nil
}

// ApplyPattern configures and starts a hardware animation.
func (receiver *InariBlueDevice) ApplyPattern(pattern patterns.Pattern) error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if err := receiver.usable(); err != nil {
		return err
	}
	if err := pattern.Validate(); err != nil {
		return err
	}

	rise, hold := encodeDuration(pattern.Rise), encodeDuration(pattern.Hold)
	decay, off, delay := encodeDuration(pattern.Decay), encodeDuration(pattern.Off), encodeDuration(pattern.Delay)

	if err := receiver.disable(); err != nil {
		return err
	}
	if err := receiver.writeRegister(inariBlueRegisterGlobalControl, inariBlueControllerEnabled); err != nil {
		return err
	}

	for channel := range inariBlueLEDChannelCount {
		timing := inariBlueRegisterChannelTimingBase + byte(channel*3)
		config := inariBlueRegisterChannelConfigBase + byte(channel)
		brightness := inariBlueRegisterChannelPWMBase + byte(channel)

		writes := [][2]byte{
			{timing, (rise << 4) | hold},
			{timing + 1, (decay << 4) | off},
			{timing + 2, (delay << 4) | pattern.Repeat},
			{brightness, pattern.Brightness},
			{config, inariBlueChannelPatternMode},
		}

		for _, write := range writes {
			if err := receiver.writeRegister(write[0], write[1]); err != nil {
				return err
			}
		}
	}
	if err := receiver.writeRegister(inariBlueRegisterGlobalControl, inariBlueControllerDisabled); err != nil {
		return err
	}

	time.Sleep(inariBlueControllerRestartDelay)
	if err := receiver.writeRegister(inariBlueRegisterGlobalControl, inariBlueControllerEnabled); err != nil {
		return err
	}

	return receiver.writeRegister(inariBlueRegisterChannelEnable, inariBlueAllChannelsEnabled)
}

// Disable turns off LED outputs without releasing the device.
func (receiver *InariBlueDevice) Disable() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if err := receiver.usable(); err != nil {
		return err
	}

	return receiver.disable()
}

func (receiver *InariBlueDevice) usable() error {
	if receiver.closed {
		return ErrClosed
	}
	if !receiver.busConnection.Connected() {
		return errors.New("lost Symbiosis D-Bus connection")
	}

	return nil
}

func (receiver *InariBlueDevice) disable() error {
	return receiver.writeRegister(inariBlueRegisterChannelEnable, inariBlueAllChannelsDisabled)
}

func (receiver *InariBlueDevice) writeRegister(register, value byte) error {
	count, err := receiver.deviceFile.Write([]byte{register, value})
	if err != nil {
		return fmt.Errorf("write LED register 0x%02x: %w", register, err)
	}

	if count != 2 {
		return fmt.Errorf("write LED register 0x%02x: %w", register, io.ErrShortWrite)
	}

	return nil
}

func encodeDuration(duration time.Duration) byte {
	value := math.Round(math.Sqrt(float64(duration) / float64(inariBlueTimingUnit)))

	return byte(math.Max(0, math.Min(15, value)))
}

// Release returns I2C ownership without changing LED state.
func (receiver *InariBlueDevice) Release() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	return receiver.release()
}

// Shutdown disables LEDs and returns I2C ownership.
func (receiver *InariBlueDevice) Shutdown() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return nil
	}

	return errors.Join(receiver.disable(), receiver.release())
}

func (receiver *InariBlueDevice) release() error {
	if receiver.closed {
		return nil
	}

	if receiver.monitorCancel != nil {
		receiver.monitorCancel()
		<-receiver.monitorDone
	}

	receiver.closed = true
	receiver.doneOnce.Do(func() { close(receiver.done) })
	receiver.stopMonitoring()

	if receiver.deviceFile == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	fileErr := receiver.deviceFile.Close()
	returnErr := receiver.busConnection.Object(tohService, tohObject).CallWithContext(ctx, tohInterface+".ReturnI2cDevAccess", 0).Err

	return errors.Join(fileErr, returnErr)
}

func (receiver *InariBlueDevice) Done() <-chan struct{} {
	return receiver.done
}

func (receiver *InariBlueDevice) startMonitoring() error {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan *dbus.Signal, 16)
	receiver.busConnection.Signal(signals)

	ownerMatch := []dbus.MatchOption{
		dbus.WithMatchSender(dbusService),
		dbus.WithMatchInterface(dbusInterface),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, tohService),
	}
	removalMatch := []dbus.MatchOption{
		dbus.WithMatchObjectPath(tohRootPath),
		dbus.WithMatchInterface(objectManagerInterface),
		dbus.WithMatchMember("InterfacesRemoved"),
	}

	if err := receiver.busConnection.AddMatchSignal(ownerMatch...); err != nil {
		cancel()
		receiver.busConnection.RemoveSignal(signals)

		return fmt.Errorf("subscribe to Symbiosis owner changes: %w", err)
	}
	if err := receiver.busConnection.AddMatchSignal(removalMatch...); err != nil {
		_ = receiver.busConnection.RemoveMatchSignal(ownerMatch...)
		cancel()
		receiver.busConnection.RemoveSignal(signals)

		return fmt.Errorf("subscribe to TOH removal: %w", err)
	}

	receiver.monitorCancel, receiver.monitorDone = cancel, make(chan struct{})
	go func() {
		defer close(receiver.monitorDone)
		defer receiver.busConnection.RemoveSignal(signals)
		defer receiver.busConnection.RemoveMatchSignal(ownerMatch...)
		defer receiver.busConnection.RemoveMatchSignal(removalMatch...)

		for {
			select {
			case <-ctx.Done():
				return
			case signal, ok := <-signals:
				if !ok {
					receiver.markUnavailable()
					return
				}
				if signal == nil {
					continue
				}

				switch signal.Name {
				case dbusInterface + ".NameOwnerChanged":
					if len(signal.Body) == 3 {
						name, _ := signal.Body[0].(string)
						oldOwner, _ := signal.Body[1].(string)

						if name == tohService && oldOwner != "" {
							receiver.markUnavailable()
							return
						}
					}
				case objectManagerInterface + ".InterfacesRemoved":
					if len(signal.Body) >= 1 {
						path, ok := signal.Body[0].(dbus.ObjectPath)
						if ok && path == tohObject {
							receiver.markUnavailable()
							return
						}
					}
				}
			}
		}
	}()
	return nil
}
func (receiver *InariBlueDevice) markUnavailable() {
	receiver.doneOnce.Do(func() {
		close(receiver.done)
	})
}
func (receiver *InariBlueDevice) stopMonitoring() {
	if receiver.monitorCancel != nil {
		receiver.monitorCancel()
		<-receiver.monitorDone
	}
}
