package main

import (
	"InariBlueNotifier/availability"
	"InariBlueNotifier/config"
	appdbus "InariBlueNotifier/dbus"
	"InariBlueNotifier/led"
	"InariBlueNotifier/patterns"
	"context"
	"errors"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

func start(controller led.Controller, sessionBus *dbus.Conn) (cleanup func() error, err error) {
	externalController := appdbus.NewExternalController(controller)

	if err := externalController.Register(sessionBus); err != nil {
		return nil, fmt.Errorf("register external controller: %w", err)
	}

	mainController := appdbus.NewController(sessionBus, controller)
	if err := mainController.Start(); err != nil {
		cleanupErr := externalController.Unregister(sessionBus)
		return nil, errors.Join(
			fmt.Errorf("start main controller: %w", err),
			cleanupErr,
		)
	}

	return func() error {
		return errors.Join(
			mainController.Stop(),
			externalController.Unregister(sessionBus),
		)
	}, nil
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	cfg, err := config.New()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to load configuration: %w", err))
	}

	sessionBus, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatalf("failed to connect to session bus: %v", err)
	}
	defer sessionBus.Close()
	systemBus, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Fatalf("failed to connect to system bus: %v", err)
	}
	defer systemBus.Close()

	checker := availability.NewChecker(sessionBus, systemBus)

	availabilityChanges := make(chan bool, 1)
	watcherErrors := make(chan error, 1)

	go func() {
		watcherErrors <- checker.Watch(ctx, availabilityChanges)
	}()

	var controllerDone <-chan struct{}
	var ledController led.Controller
	var stopServices func() error

	cleanupBase := func(shutdowner func(ledController led.Controller) error) {
		if stopServices != nil {
			if err := stopServices(); err != nil {
				log.Printf("failed to stop services: %v", err)
			}
			stopServices = nil
		}

		if ledController != nil {
			if err := shutdowner(ledController); err != nil {
				log.Printf("failed to release LED controller: %v", err)
			}
			ledController = nil
		}

		controllerDone = nil
	}
	cleanup := func() {
		cleanupBase(led.Controller.Release)
	}
	shutdown := func() {
		cleanupBase(led.Controller.Shutdown)
	}

	defer shutdown()

	var available bool
	var retry <-chan time.Time

	log.Println("LED daemon started")

	for {
		select {
		case isAvailable := <-availabilityChanges:
			available = isAvailable

			if !available {
				retry = nil
				cleanup()
				log.Println("Original Inari Blue behavior enabled")

				continue
			}

			retry = time.After(2 * time.Second)

			if ledController != nil {
				continue
			}

			var err error
			ledController, err = led.NewWithPatterns(
				systemBus,
				patterns.MediumLengthBreath,
				cfg.CallPattern,
			)
			if err != nil {
				log.Printf("failed to create LED controller: %v", err)
				continue
			}

			stopServices, err = start(ledController, sessionBus)
			if err != nil {
				log.Printf("failed to start services: %v", err)
				cleanup()
				continue
			}
			controllerDone = ledController.Done()

			log.Println("LED controller acquired")

		case <-controllerDone:
			log.Println("LED controller lost")
			cleanup()

			if available {
				retry = time.After(5 * time.Second)
			}

		case err := <-watcherErrors:
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("availability watcher failed: %v", err)
			}
			return

		case <-ctx.Done():
			log.Println("LED daemon stopping")
			return
		case <-retry:
			retry = nil

			if !available || ledController != nil {
				continue
			}

			newController, err := led.NewWithPatterns(
				systemBus,
				patterns.DefaultNotificationPattern,
				cfg.CallPattern,
			)
			if err != nil {
				log.Printf("failed to create LED controller: %v", err)
				retry = time.After(5 * time.Second)
				continue
			}

			ledController = newController

			stopServices, err = start(ledController, sessionBus)
			if err != nil {
				log.Printf("failed to start services: %v", err)
				cleanup()
				retry = time.After(5 * time.Second)
				continue
			}

			controllerDone = ledController.Done()
			log.Println("LED controller acquired")
		}
	}
}
