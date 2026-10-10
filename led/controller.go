package led

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
	dbusService            = "org.freedesktop.DBus"
	dbusInterface          = "org.freedesktop.DBus"
	objectManagerInterface = "org.freedesktop.DBus.ObjectManager"

	symbiosisRootPath = "/org/sailfishos/tohd1"

	symbiosisService   = "org.sailfishos.tohd1"
	symbiosisObject    = "/org/sailfishos/tohd1/toh"
	symbiosisInterface = "org.sailfishos.tohd1.Toh"

	i2cDeviceAddress = 0x45
	i2cSlaveIOCTL    = 0x0703

	registerGlobalControl     byte = 0x01
	registerChannelEnable     byte = 0x30
	registerChannelConfigBase byte = 0x31
	registerChannelPWMBase    byte = 0x34
	registerChannelTimingBase byte = 0x37

	ledChannelCount = 3

	controllerEnabled  byte = 0x01
	controllerDisabled byte = 0x00

	// CUR=15, SYNC=1, MD=1.
	channelPatternMode byte = 0x9f

	allChannelsEnabled  byte = 0x07
	allChannelsDisabled byte = 0x00

	controllerRestartDelay = 200 * time.Millisecond
)

var ErrClosed = errors.New("LED controller is closed")

type State uint8

const (
	Off State = iota
	Breathing
	Blinking
)

type Controller interface {
	SetState(state State) error
	Release() error
	Shutdown() error
	Done() <-chan struct{}
}

type controller struct {
	mutex sync.Mutex

	busConnection *dbus.Conn
	deviceFile    *os.File

	notificationPattern patterns.Pattern
	callPattern         patterns.Pattern

	currentState State
	stateApplied bool
	closed       bool

	done          chan struct{}
	doneOnce      sync.Once
	monitorCancel context.CancelFunc
	monitorDone   chan struct{}
}

// New borrows access to the TOH I2C bus from Symbiosis.
//
// The caller must be authorized by Symbiosis configuration
// or run as root.
//
// The controller does not modify LED registers until SetState.
func New(connection *dbus.Conn) (Controller, error) {
	return NewWithPatterns(
		connection,
		patterns.MediumLengthBreath,
		patterns.SlowBlink,
	)
}

func NewWithPatterns(
	connection *dbus.Conn,
	notificationPattern patterns.Pattern,
	callPattern patterns.Pattern,
) (Controller, error) {
	return NewWithContext(
		context.Background(),
		connection,
		notificationPattern,
		callPattern,
	)
}

func NewWithContext(
	ctx context.Context,
	connection *dbus.Conn,
	notificationPattern patterns.Pattern,
	callPattern patterns.Pattern,
) (Controller, error) {
	if err := notificationPattern.Validate(); err != nil {
		return nil, fmt.Errorf("notification pattern: %w", err)
	}

	if err := callPattern.Validate(); err != nil {
		return nil, fmt.Errorf("call pattern: %w", err)
	}

	if !connection.SupportsUnixFDs() {
		return nil, errors.New(
			"system D-Bus does not support Unix FD passing",
		)
	}

	instance := &controller{
		busConnection:       connection,
		notificationPattern: notificationPattern,
		callPattern:         callPattern,
		done:                make(chan struct{}),
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

	service := connection.Object(
		symbiosisService,
		symbiosisObject,
	)

	// Borrow the I2C descriptor and keep TOH power enabled
	// after returning the loan.
	//
	// The latter is important when switching back to the
	// original Inari Blue breathing behavior.
	var descriptor dbus.UnixFD

	err := service.CallWithContext(
		ctx,
		symbiosisInterface+".BorrowI2cDevAccessWithPower",
		0,
		true,
	).Store(&descriptor)

	if err != nil {
		return nil, fmt.Errorf(
			"borrow TOH I2C access from Symbiosis: %w",
			err,
		)
	}

	deviceFile := os.NewFile(
		uintptr(descriptor),
		"symbiosis-toh-i2c",
	)

	// The loan is now acquired. On subsequent failures,
	// also release it.
	defer func() {
		if initialized {
			return
		}

		if deviceFile != nil {
			_ = deviceFile.Close()
		}

		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		_ = service.CallWithContext(
			cleanupCtx,
			symbiosisInterface+".ReturnI2cDevAccess",
			0,
		).Err
	}()

	if deviceFile == nil {
		return nil, errors.New("invalid I2C descriptor")
	}

	// Select the LED controller at address 0x45.
	//
	// Symbiosis must have this address registered in its
	// TOH device configuration.
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		deviceFile.Fd(),
		i2cSlaveIOCTL,
		i2cDeviceAddress,
	)

	if errno != 0 {
		return nil, fmt.Errorf(
			"select LED I2C address 0x%02x: %w",
			i2cDeviceAddress,
			errno,
		)
	}

	// Symbiosis could have disappeared while the D-Bus
	// borrowing call or ioctl was executing.
	select {
	case <-instance.Done():
		return nil, errors.New("symbiosis became unavailable during LED initialization")
	default:
	}

	instance.deviceFile = deviceFile
	initialized = true

	return instance, nil
}

// SetState configures the desired LED behavior.
//
// Repeated calls with the same successfully applied state
// do not perform additional I2C writes.
func (receiver *controller) SetState(state State) error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return ErrClosed
	}

	if !receiver.busConnection.Connected() {
		receiver.stateApplied = false
		return errors.New("lost Symbiosis D-Bus connection")
	}

	if receiver.stateApplied && receiver.currentState == state {
		return nil
	}

	var err error

	switch state {
	case Off:
		err = receiver.disableLEDs()

	case Breathing:
		err = receiver.configureAnimation(receiver.notificationPattern)

	case Blinking:
		err = receiver.configureAnimation(receiver.callPattern)

	default:
		return fmt.Errorf("unknown LED state: %d", state)
	}

	if err != nil {
		receiver.stateApplied = false
		return err
	}

	receiver.currentState = state
	receiver.stateApplied = true

	return nil
}

func (receiver *controller) disableLEDs() error {
	return receiver.writeRegister(
		registerChannelEnable,
		allChannelsDisabled,
	)
}

func (receiver *controller) configureAnimation(pattern patterns.Pattern) error {
	if err := pattern.Validate(); err != nil {
		return err
	}

	rise := encodeDuration(pattern.Rise)
	hold := encodeDuration(pattern.Hold)
	decay := encodeDuration(pattern.Decay)
	off := encodeDuration(pattern.Off)
	delay := encodeDuration(pattern.Delay)

	// Disable LED outputs before reconfiguration.
	if err := receiver.disableLEDs(); err != nil {
		return err
	}

	// Enable the controller.
	if err := receiver.writeRegister(
		registerGlobalControl,
		controllerEnabled,
	); err != nil {
		return err
	}

	// Configure all three channels identically.
	for channel := range ledChannelCount {
		timingRegister := registerChannelTimingBase + byte(channel*3)
		configRegister := registerChannelConfigBase + byte(channel)
		brightnessRegister := registerChannelPWMBase + byte(channel)

		// Rise and hold.
		if err := receiver.writeRegister(
			timingRegister,
			(rise<<4)|hold,
		); err != nil {
			return err
		}

		// Decay and off.
		if err := receiver.writeRegister(
			timingRegister+1,
			(decay<<4)|off,
		); err != nil {
			return err
		}

		// Initial delay and repeat count.
		if err := receiver.writeRegister(
			timingRegister+2,
			(delay<<4)|pattern.Repeat,
		); err != nil {
			return err
		}

		// PWM brightness.
		if err := receiver.writeRegister(
			brightnessRegister,
			pattern.Brightness,
		); err != nil {
			return err
		}

		// Hardware pattern mode.
		if err := receiver.writeRegister(
			configRegister,
			channelPatternMode,
		); err != nil {
			return err
		}
	}

	// Restart the hardware animation.
	//
	// Preserve the sequence used by the original Python
	// implementation.
	if err := receiver.writeRegister(
		registerGlobalControl,
		controllerDisabled,
	); err != nil {
		return err
	}

	time.Sleep(controllerRestartDelay)

	if err := receiver.writeRegister(
		registerGlobalControl,
		controllerEnabled,
	); err != nil {
		return err
	}

	// Enable all three output channels.
	return receiver.writeRegister(
		registerChannelEnable,
		allChannelsEnabled,
	)
}

func (receiver *controller) writeRegister(register, value byte) error {
	data := []byte{register, value}

	count, err := receiver.deviceFile.Write(data)
	if err != nil {
		return fmt.Errorf(
			"write LED register 0x%02x: %w",
			register,
			err,
		)
	}

	if count != len(data) {
		return fmt.Errorf(
			"write LED register 0x%02x: %w",
			register,
			io.ErrShortWrite,
		)
	}

	return nil
}

// encodeDuration converts a duration to the controller's
// 4-bit nonlinear timing representation.
//
// duration = 31.25ms * nibble²
func encodeDuration(duration time.Duration) byte {
	value := math.Round(
		math.Sqrt(duration.Seconds() * 1000 / 31.25),
	)

	return byte(math.Max(0, math.Min(15, value)))
}

// Release returns I2C ownership to Symbiosis without
// modifying the LED configuration.
//
// The controller cannot be reused after Release.
func (receiver *controller) Release() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	return receiver.release()
}

func (receiver *controller) release() error {
	if receiver.closed {
		return nil
	}

	if receiver.monitorCancel != nil {
		receiver.monitorCancel()
		<-receiver.monitorDone
	}

	receiver.closed = true
	receiver.doneOnce.Do(func() {
		close(receiver.done)
	})

	receiver.stopMonitoring()

	if receiver.deviceFile == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Stop using the borrowed descriptor before returning it.
	fileErr := receiver.deviceFile.Close()

	service := receiver.busConnection.Object(
		symbiosisService,
		symbiosisObject,
	)

	returnErr := service.CallWithContext(
		ctx,
		symbiosisInterface+".ReturnI2cDevAccess",
		0,
	).Err

	return errors.Join(
		fileErr,
		returnErr,
	)
}

// Shutdown disables LED outputs and returns access to Symbiosis.
//
// Use Release instead when yielding control to the original
// Inari Blue behavior without touching the hardware state.
func (receiver *controller) Shutdown() error {
	receiver.mutex.Lock()
	defer receiver.mutex.Unlock()

	if receiver.closed {
		return nil
	}

	disableErr := receiver.disableLEDs()
	releaseErr := receiver.release()

	return errors.Join(disableErr, releaseErr)
}

func (receiver *controller) Done() <-chan struct{} {
	return receiver.done
}

func (receiver *controller) startMonitoring() error {
	ctx, cancel := context.WithCancel(context.Background())

	signals := make(chan *dbus.Signal, 16)
	receiver.busConnection.Signal(signals)

	ownerMatch := []dbus.MatchOption{
		dbus.WithMatchSender(dbusService),
		dbus.WithMatchInterface(dbusInterface),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, symbiosisService),
	}

	removalMatch := []dbus.MatchOption{
		dbus.WithMatchObjectPath(symbiosisRootPath),
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

	receiver.monitorCancel = cancel
	receiver.monitorDone = make(chan struct{})

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
					if len(signal.Body) != 3 {
						continue
					}

					name, _ := signal.Body[0].(string)
					oldOwner, _ := signal.Body[1].(string)

					if name == symbiosisService && oldOwner != "" {
						receiver.markUnavailable()
						return
					}

				case objectManagerInterface + ".InterfacesRemoved":
					if len(signal.Body) < 1 {
						continue
					}

					path, ok := signal.Body[0].(dbus.ObjectPath)
					if ok && path == symbiosisObject {
						receiver.markUnavailable()
						return
					}
				}
			}
		}
	}()

	return nil
}

func (receiver *controller) markUnavailable() {
	receiver.doneOnce.Do(func() {
		close(receiver.done)
	})
}

func (receiver *controller) stopMonitoring() {
	if receiver.monitorCancel == nil {
		return
	}

	receiver.monitorCancel()
	<-receiver.monitorDone
}
