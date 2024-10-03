package app

import (
	"wails-scaffold.iserranodev.net/internal/models"
)

// Greet returns a greeting for the given name
func (a *App) LoadHosts() ([]models.HostRecord, error) {
	hosts, err := a.hostRecords.GetHostRecords()
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

// Greet returns a greeting for the given name
func (a *App) SaveHost(ID string, Host string, Port int, Name string, User string, Pass string) (bool, string) {

	if ID != "0" {
		_, err := a.hostRecords.EditHostRecord(ID, Host, Port, Name, User, Pass)
		if err != nil {
			return false, err.Error()
		}
	} else {
		_, err := a.hostRecords.SaveHostRecord(Host, Port, Name, User, Pass)
		if err != nil {
			return false, err.Error()
		}
	}

	return true, "Registered successfully"
}

// Greet returns a greeting for the given name
func (a *App) RemoveHost(ID string) (bool, string) {

	_, err := a.hostRecords.RemoveHostRecord(ID)
	if err != nil {
		return false, err.Error()
	}

	return true, "Removed successfully"
}
