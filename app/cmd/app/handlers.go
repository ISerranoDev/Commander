package app

import (
	"fmt"
	"wails-scaffold.iserranodev.net/internal/models"
)

func (a *App) LoadHosts() ([]models.HostRecord, error) {
	hosts, err := a.hostRecords.GetHostRecords()
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

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

func (a *App) RemoveHost(ID string) (bool, string) {

	_, err := a.hostRecords.RemoveHostRecord(ID)
	if err != nil {
		return false, err.Error()
	}

	return true, "Removed successfully"
}

func (a *App) StartSSHSession(hostID string) error {
	sshSession, err := a.sshSessions.StartSSHSession(hostID)
	if err != nil {
		return err
	}
	a.sshSession = sshSession
	return nil
}

func (a *App) SendSSHCommand(command string) (string, error) {
	if a.sshSession == nil {
		return "", fmt.Errorf("no SSH session started")
	}
	return a.sshSession.SendCommand(command)
}

func (a *App) CloseSSHSession() error {
	// Verificar si hay una sesión SSH activa
	if a.sshSession == nil {
		return fmt.Errorf("no hay una sesión SSH activa para cerrar")
	}

	// Cerrar la sesión SSH y el cliente
	a.sshSession.Close()
	a.sshSession = nil

	return nil
}
