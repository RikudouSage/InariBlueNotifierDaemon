// Package knock plays an LED pattern when traffic reaches a configured port.
package knock

import (
	"InariBlueNotifier/config"
	"InariBlueNotifier/led"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

var ErrDisabled = errors.New("knock is not enabled, refusing to start")

// Knock manages the lifetime of the configured network listeners.
type Knock interface {
	StartListening() error
	StopListening() error
}

type knock struct {
	config     *config.Config
	controller led.Controller

	mu        sync.Mutex
	listeners []net.Listener
	packets   []net.PacketConn
	done      chan struct{}
	workers   sync.WaitGroup
}

func New(config *config.Config, controller led.Controller) Knock {
	return &knock{
		config:     config,
		controller: controller,
	}
}

// StartListening binds every configured protocol before starting any workers.
// Addresses select local interfaces to bind; an empty list binds all interfaces.
func (receiver *knock) StartListening() error {
	receiver.mu.Lock()
	defer receiver.mu.Unlock()

	if receiver.config == nil {
		return errors.New("knock config is nil")
	}
	if !receiver.config.Knock.Enabled {
		return ErrDisabled
	}
	if receiver.done != nil {
		return errors.New("knock is already listening")
	}
	if receiver.controller == nil {
		return errors.New("knock LED controller is nil")
	}

	settings := receiver.config.Knock
	if len(settings.Protocols) == 0 {
		return errors.New("no knock protocols configured")
	}
	if err := settings.Pattern.Validate(); err != nil {
		return fmt.Errorf("invalid knock pattern: %w", err)
	}
	if settings.Timeout < 0 {
		return errors.New("knock timeout cannot be negative")
	}

	addresses := make([]string, 0, len(settings.Addresses))
	seenAddresses := make(map[netip.Addr]bool, len(settings.Addresses))
	for _, raw := range settings.Addresses {
		address, err := netip.ParseAddr(raw)
		if err != nil {
			return fmt.Errorf("invalid knock bind address %q: %w", raw, err)
		}

		address = address.Unmap()
		if seenAddresses[address] {
			return fmt.Errorf("duplicate knock bind address %q", raw)
		}

		seenAddresses[address] = true
		addresses = append(addresses, address.String())
	}
	if len(addresses) == 0 {
		addresses = append(addresses, "")
	}

	listeners := make([]net.Listener, 0, len(addresses))
	packets := make([]net.PacketConn, 0, len(addresses))
	closeSockets := func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
		for _, packet := range packets {
			_ = packet.Close()
		}
	}

	seen := make(map[string]bool, len(settings.Protocols))
	port := settings.Port
	for _, protocol := range settings.Protocols {
		if protocol != "tcp" && protocol != "udp" {
			closeSockets()
			return fmt.Errorf("unsupported knock protocol %q", protocol)
		}

		if seen[protocol] {
			closeSockets()
			return fmt.Errorf("duplicate knock protocol %q", protocol)
		}

		seen[protocol] = true
		for _, host := range addresses {
			address := net.JoinHostPort(host, strconv.Itoa(int(port)))
			network := protocol

			if host != "" {
				if netip.MustParseAddr(host).Is4() {
					network += "4"
				} else {
					network += "6"
				}
			}

			if protocol == "tcp" {
				listener, err := net.Listen(network, address)
				if err != nil {
					closeSockets()
					return fmt.Errorf("listen for TCP knocks on %s: %w", address, err)
				}

				listeners = append(listeners, listener)
				if port == 0 {
					port = uint16(listener.Addr().(*net.TCPAddr).Port)
				}
			} else {
				packet, err := net.ListenPacket(network, address)
				if err != nil {
					closeSockets()
					return fmt.Errorf("listen for UDP knocks on %s: %w", address, err)
				}

				packets = append(packets, packet)
				if port == 0 {
					port = uint16(packet.LocalAddr().(*net.UDPAddr).Port)
				}
			}
		}
	}

	receiver.listeners, receiver.packets = listeners, packets
	receiver.done = make(chan struct{})
	done := receiver.done
	events := make(chan struct{})

	receiver.workers.Go(func() {
		receiver.playKnocks(done, events, settings)
	})

	for _, listener := range listeners {
		receiver.workers.Go(func() {
			receiver.listenTCP(done, events, listener)
		})
	}
	for _, packet := range packets {
		receiver.workers.Go(func() {
			receiver.listenUDP(done, events, packet)
		})
	}

	return nil
}

// StopListening closes the sockets and waits until no more patterns can be played.
// It leaves the current LED state alone and can be followed by another start.
func (receiver *knock) StopListening() error {
	receiver.mu.Lock()
	defer receiver.mu.Unlock()

	if receiver.done == nil {
		return nil
	}
	close(receiver.done)

	var closeErrors []error
	for _, listener := range receiver.listeners {
		closeErrors = append(closeErrors, listener.Close())
	}
	for _, packet := range receiver.packets {
		closeErrors = append(closeErrors, packet.Close())
	}

	receiver.workers.Wait()
	receiver.listeners, receiver.packets, receiver.done = nil, nil, nil

	return errors.Join(closeErrors...)
}

func (*knock) notify(done <-chan struct{}, events chan<- struct{}) bool {
	select {
	case <-done:
		return false
	case events <- struct{}{}:
		return true
	}
}

func (receiver *knock) listenTCP(done <-chan struct{}, events chan<- struct{}, listener net.Listener) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("knock TCP listener stopped: %v", err)
			}
			return
		}
		_ = connection.Close()
		if !receiver.notify(done, events) {
			return
		}
	}
}

func (receiver *knock) listenUDP(done <-chan struct{}, events chan<- struct{}, packet net.PacketConn) {
	buffer := make([]byte, 1)
	for {
		_, _, err := packet.ReadFrom(buffer)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("knock UDP listener stopped: %v", err)
			}
			return
		}
		if !receiver.notify(done, events) {
			return
		}
	}
}

func (receiver *knock) playKnocks(done <-chan struct{}, events <-chan struct{}, settings config.Knock) {
	var timer *time.Timer
	var timeout <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-done:
			return
		case <-events:
			select {
			case <-done:
				return
			default:
			}
			if err := receiver.controller.PlayPattern(settings.Pattern); err != nil {
				log.Printf("failed to play knock pattern: %v", err)
				continue
			}
			if settings.Timeout > 0 {
				if timer == nil {
					timer = time.NewTimer(settings.Timeout)
				} else {
					timer.Reset(settings.Timeout)
				}
				timeout = timer.C
			}
		case <-timeout:
			timeout = nil
			if err := receiver.controller.Stop(); err != nil {
				log.Printf("failed to stop knock pattern: %v", err)
			}
		}
	}
}
