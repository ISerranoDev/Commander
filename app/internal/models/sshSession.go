package models

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"time"
)

type SSHSessionInterface interface {
	StartSSHSession(ID string) (*SSHSession, error)
	SendCommand(command string) (string, error)
	Close()
}

type SSHSession struct {
	Client  *ssh.Client
	Session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
}

type SSHSessionModel struct {
	SshSessions []*SSHSession
}

func (s *SSHSessionModel) StartSSHSession(ID string) (*SSHSession, error) {
	// Cargar los registros de hosts
	hostRecords, err := LoadFromFile()
	if err != nil {
		return nil, err
	}

	// Buscar el registro por ID
	var targetHost HostRecord
	found := false
	for _, record := range hostRecords {
		if record.ID == ID {
			targetHost = record
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("host with ID %s not found", ID)
	}

	// Configuración SSH
	config := &ssh.ClientConfig{
		User: targetHost.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(targetHost.Pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second, // Tiempo de espera
	}

	// Establecer conexión
	address := fmt.Sprintf("%s:%d", targetHost.Host, targetHost.Port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", targetHost.Host, err)
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	// Crear un canal para comandos interactivos
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Modo de terminal
	if err := session.RequestPty("xterm", 80, 40, ssh.TerminalModes{}); err != nil {
		return nil, err
	}

	// Iniciar shell interactivo
	if err := session.Shell(); err != nil {
		return nil, err
	}

	sshSession := &SSHSession{
		Client:  client,
		Session: session,
		Stdin:   stdin,
		Stdout:  stdout,
	}

	// Guardar la sesión
	s.SshSessions = append(s.SshSessions, sshSession)

	return sshSession, nil
}

// Función para enviar comandos a una sesión SSH
func (s *SSHSession) SendCommand(command string) (string, error) {
	_, err := s.Stdin.Write([]byte(command + "\n"))
	if err != nil {
		return "", err
	}

	// Leer la respuesta
	buf := make([]byte, 1024)
	n, err := s.Stdout.Read(buf)
	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil
}

// Cerrar sesión SSH
func (s *SSHSession) Close() {
	s.Session.Close()
	s.Client.Close()
}
