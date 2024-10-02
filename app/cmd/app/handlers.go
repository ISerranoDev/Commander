package app

import "wails-scaffold.iserranodev.net/internal/models"

// Greet returns a greeting for the given name
func (a *App) LoadHosts() []models.HostRecord {
	hosts, _ := a.hostRecords.GetHostRecords()

	return hosts
}
