package models

type HostRecordInterface interface {
	GetHostRecords() ([]HostRecord, error)
}

type HostRecord struct {
	ID   string
	Host string
	Name string
	User string
	Pass string
	Port int
}

type HostRecordModel struct {
}

func (h *HostRecordModel) GetHostRecords() ([]HostRecord, error) {
	records := []HostRecord{
		{"16091dc9-ad78-466a-ac97-fca468349a22", "192.168.1.1", "Server1", "admin", "password1", 22},
		{"1187a90b-d456-4d28-bad0-f68c53d9110f", "192.168.1.2", "Server2", "admin", "password2", 22},
		{"e9e37447-aff5-4512-9cf8-10a545c0004d", "192.168.1.3", "Server3", "admin", "password3", 22},
		{"bf484a64-b188-4170-b653-53f043960b23", "192.168.1.4", "Server4", "admin", "password4", 22},
		{"73a3ddb5-cb83-4a4c-bd34-d3b4355e55d6", "192.168.1.5", "Server5", "admin", "password5", 22},
		{"e9648f8d-08b9-4ac9-aa6d-7b09d01830a6", "192.168.1.6", "Server6", "admin", "password6", 22},
		{"7224b843-43f5-4c3f-aa87-2238fc1a96c3", "192.168.1.7", "Server7", "admin", "password7", 22},
		{"70966f6f-42b5-43db-9474-fea472bb5684", "192.168.1.8", "Server8", "admin", "password8", 22},
		{"e27423ae-6dd4-4347-9cfc-204887c3c399", "192.168.1.9", "Server9", "admin", "password9", 22},
		{"98aded55-db09-4996-870d-577aae0462e8", "192.168.1.10", "Server10", "admin", "password10", 22},
	}

	return records, nil
}
