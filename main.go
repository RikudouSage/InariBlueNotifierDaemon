package main

import (
	"InariBlueNotifier/availability"
	"InariBlueNotifier/config"
	appdbus "InariBlueNotifier/dbus"
	"InariBlueNotifier/devices"
	"InariBlueNotifier/knock"
	"InariBlueNotifier/led"
	"InariBlueNotifier/migrations"
	"InariBlueNotifier/notifications"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

func start(
	controller led.Controller,
	sessionBus, systemBus *dbus.Conn,
	cfg *config.Config,
	logger *config.Logger,
) (cleanup func() error, err error) {
	externalController := appdbus.NewExternalController(controller, logger)
	notificationMatcher := notifications.DefaultMatcher(cfg)
	mainController := appdbus.NewControllerWithConfig(sessionBus, systemBus, controller, cfg, notificationMatcher, logger)
	knockEvents := &knockEventController{controller: mainController, external: externalController}
	knockController := knock.New(cfg, knockEvents, func(err error) {
		externalController.UpdateKnockStatus(appdbus.KnockFailed, err.Error())
	}, logger)
	mainController.SetKnockClearHandler(knockController.ClearActive)
	if cfg.Knock.Enabled {
		externalController.UpdateKnockStatus(appdbus.KnockStarting, "")
	}

	cleanup = func() error {
		controllerErr := mainController.Stop()
		knockErr := knockController.StopListening()
		if cfg.Knock.Enabled {
			externalController.UpdateKnockStatus(appdbus.KnockStopped, "")
		}

		return errors.Join(
			knockErr,
			controllerErr,
			externalController.Unregister(sessionBus),
		)
	}

	if err := externalController.Register(sessionBus); err != nil {
		return cleanup, fmt.Errorf("register external controller: %w", err)
	}

	if err := mainController.Start(); err != nil {
		cleanupErr := externalController.Unregister(sessionBus)
		return cleanup, errors.Join(
			fmt.Errorf("start main controller: %w", err),
			cleanupErr,
		)
	}

	if cfg.Knock.Enabled {
		if err := knockController.StartListening(); err != nil {
			logger.Error("failed to start knock listener", slog.String("error", err.Error()))
			externalController.UpdateKnockStatus(appdbus.KnockFailed, err.Error())
		} else {
			externalController.KnockListening()
		}
	}

	return cleanup, nil
}

type knockEventController struct {
	controller appdbus.Controller
	external   *appdbus.ExternalController
}

func (receiver *knockEventController) SetKnockActive(active bool) error {
	err := receiver.controller.SetKnockActive(active)
	receiver.external.UpdateKnockActive(active)
	return err
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer stop()

	logger := config.NewLogger()
	defer logger.Close()

	cfg, err := config.New()
	if err != nil {
		logger.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	migrator := migrations.GetMigrator()
	if err = migrator.Migrate(cfg); err != nil {
		logger.Error("failed to migrate", slog.String("error", err.Error()))
		os.Exit(1)
	}

	sessionBus, err := dbus.ConnectSessionBus()
	if err != nil {
		logger.Error("failed to connect session bus", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer sessionBus.Close()
	systemBus, err := dbus.ConnectSystemBus()
	if err != nil {
		logger.Error("failed to connect system bus", slog.String("error", err.Error()))
		os.Exit(1)
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
				logger.Warn("failed to stop services", slog.String("error", err.Error()))
			}
			stopServices = nil
		}

		if ledController != nil {
			if err := shutdowner(ledController); err != nil {
				logger.Warn("failed to release LED controller:", slog.String("error", err.Error()))
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

	logger.Info("LED daemon started")

	for {
		select {
		case isAvailable := <-availabilityChanges:
			available = isAvailable

			if !available {
				retry = nil
				cleanup()
				logger.Info("Original Inari Blue behavior enabled")

				continue
			}

			retry = time.After(2 * time.Second)

			if ledController != nil {
				continue
			}

			device, err := devices.NewInariBlueDevice(systemBus)
			if err != nil {
				logger.Error("failed to create Inari Blue device", slog.String("error", err.Error()))
				continue
			}

			ledController, err = led.New(device)
			if err != nil {
				logger.Error("failed to create LED controller", slog.String("error", err.Error()))
				_ = device.Release()
				continue
			}

			stopServices, err = start(ledController, sessionBus, systemBus, cfg, logger)
			if err != nil {
				logger.Error("failed to start services", slog.String("error", err.Error()))
				cleanup()
				continue
			}
			controllerDone = ledController.Done()

			logger.Info("LED controller acquired")

		case <-controllerDone:
			logger.Warn("LED controller lost")
			cleanup()

			if available {
				retry = time.After(5 * time.Second)
			}

		case err := <-watcherErrors:
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("availability watcher failed", slog.String("error", err.Error()))
			}
			return

		case <-ctx.Done():
			logger.Info("LED daemon stopping")
			return
		case <-retry:
			retry = nil

			if !available || ledController != nil {
				continue
			}

			device, err := devices.NewInariBlueDevice(systemBus)
			if err != nil {
				logger.Error("failed to create Inari Blue device", slog.String("error", err.Error()))
				retry = time.After(5 * time.Second)
				continue
			}

			newController, err := led.New(device)
			if err != nil {
				logger.Error("failed to create LED controller", slog.String("error", err.Error()))
				_ = device.Release()
				retry = time.After(5 * time.Second)
				continue
			}

			ledController = newController

			stopServices, err = start(ledController, sessionBus, systemBus, cfg, logger)
			if err != nil {
				logger.Error("failed to start services", slog.String("error", err.Error()))
				cleanup()
				retry = time.After(5 * time.Second)
				continue
			}

			controllerDone = ledController.Done()
			logger.Info("LED controller acquired")
		}
	}
}
