package modules

import (
	"fmt"
	"log"

	"github.com/kardianos/service"
)

// ServiceConfig holds the service registration configuration.
type ServiceConfig struct {
	Name        string
	DisplayName string
	Description string
	Arguments   []string
}

// Program implements service.Interface for kardianos/service.
type Program struct {
	StartFunc func() error
	StopFunc  func() error
}

// Start is called when the service starts.
func (p *Program) Start(_ service.Service) error {
	go func() {
		if err := p.StartFunc(); err != nil {
			log.Fatalf("Failed to start: %v", err)
		}
	}()
	return nil
}

// Stop is called when the service stops.
func (p *Program) Stop(_ service.Service) error {
	if p.StopFunc != nil {
		return p.StopFunc()
	}
	return nil
}

// InstallService registers the program as an OS service.
func InstallService(cfg ServiceConfig) error {
	svcConfig := &service.Config{
		Name:        cfg.Name,
		DisplayName: cfg.DisplayName,
		Description: cfg.Description,
		Arguments:   cfg.Arguments,
	}
	prg := &Program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	return s.Install()
}

// UninstallService removes the OS service registration.
func UninstallService() error {
	svcConfig := &service.Config{
		Name: "DevGatewayDNS",
	}
	prg := &Program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	return s.Uninstall()
}

// StartService starts the registered OS service.
func StartService() error {
	svcConfig := &service.Config{
		Name: "DevGatewayDNS",
	}
	prg := &Program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	return s.Start()
}

// StopService stops the registered OS service.
func StopService() error {
	svcConfig := &service.Config{
		Name: "DevGatewayDNS",
	}
	prg := &Program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	return s.Stop()
}

// ServiceStatus returns the status of the OS service.
func ServiceStatus() (string, error) {
	svcConfig := &service.Config{
		Name: "DevGatewayDNS",
	}
	prg := &Program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create service: %w", err)
	}
	st, err := s.Status()
	if err != nil {
		return "", err
	}
	switch st {
	case service.StatusRunning:
		return "running", nil
	case service.StatusStopped:
		return "stopped", nil
	default:
		return "unknown", nil
	}
}
