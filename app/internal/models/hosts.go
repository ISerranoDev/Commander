package models

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
)

type HostRecordInterface interface {
	GetHostRecords() ([]HostRecord, error)
	SaveHostRecord(Host string, Port int, Name string, User string, Pass string) ([]HostRecord, error)
	EditHostRecord(ID string, Host string, Port int, Name string, User string, Pass string) ([]HostRecord, error)
	RemoveHostRecord(ID string) (bool, error)
}

type HostRecord struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
	Name string `json:"name"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

type HostRecordModel struct {
	Hosts []HostRecord `json:"hosts"`
}

// Obtiene la ruta de hosts.json en el mismo directorio que el ejecutable
func getHostsFilePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exePath), "hosts.json"), nil
}

func SaveToFile(records []HostRecord) error {

	hostsFilePath, err := getHostsFilePath()
	if err != nil {
		return err
	}

	file, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(hostsFilePath, file, 0644)
}

func LoadFromFile() ([]HostRecord, error) {
	hostsFilePath, err := getHostsFilePath()
	if err != nil {
		return nil, err
	}

	file, err := os.ReadFile(hostsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []HostRecord{}, nil // Retorna un slice vacío si el archivo no existe
		}
		return nil, err
	}

	var records []HostRecord
	err = json.Unmarshal(file, &records)
	if err != nil {
		return nil, err
	}

	return records, nil
}

func getBaseDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Obtener el directorio donde está el ejecutable
	return filepath.Dir(exePath), nil
}

func (h *HostRecordModel) GetHostRecords() ([]HostRecord, error) {
	records, err := LoadFromFile()
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (h *HostRecordModel) SaveHostRecord(Host string, Port int, Name string, User string, Pass string) ([]HostRecord, error) {

	if Host == "" || Name == "" || User == "" || Pass == "" {
		return nil, fmt.Errorf("all fields must be provided")
	}

	hostRecords, err := LoadFromFile()
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()

	// Crear un nuevo registro
	newRecord := HostRecord{
		ID:   id,
		Host: Host,
		Port: Port,
		Name: Name,
		User: User,
		Pass: Pass,
	}

	// Agregarlo al slice
	hostRecords = append(hostRecords, newRecord)

	// Guardar nuevamente todos los registros en el archivo
	err = SaveToFile(hostRecords)
	if err != nil {
		return nil, err
	}

	return hostRecords, nil

}

func (h *HostRecordModel) EditHostRecord(id string, Host string, Port int, Name string, User string, Pass string) ([]HostRecord, error) {
	// Cargar los registros existentes del archivo
	hostRecords, err := LoadFromFile()
	if err != nil {
		return nil, err
	}

	recordFound := false
	for i, record := range hostRecords {
		if record.ID == id {
			hostRecords[i].Host = Host
			hostRecords[i].Port = Port
			hostRecords[i].Name = Name
			hostRecords[i].User = User
			hostRecords[i].Pass = Pass
			recordFound = true
			break
		}
	}

	if !recordFound {
		return nil, fmt.Errorf("registry with ID %s was not found", id)
	}

	// Guardar los cambios nuevamente en el archivo
	err = SaveToFile(hostRecords)
	if err != nil {
		return nil, err
	}

	return hostRecords, nil
}

func (h *HostRecordModel) RemoveHostRecord(ID string) (bool, error) {
	// Cargar los registros existentes del archivo
	hostRecords, err := LoadFromFile()
	if err != nil {
		return false, err
	}

	// Buscar y eliminar el registro por ID
	recordFound := false
	for i, record := range hostRecords {
		if record.ID == ID {
			// Eliminar el registro del slice
			hostRecords = append(hostRecords[:i], hostRecords[i+1:]...)
			recordFound = true
			break
		}
	}

	if !recordFound {
		return false, fmt.Errorf("registry with ID %s was not found", ID)
	}

	// Guardar la lista actualizada en el archivo
	err = SaveToFile(hostRecords)
	if err != nil {
		return false, err
	}

	return true, nil
}
