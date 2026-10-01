package model

import (
	"errors"
	"strings"
	"time"
)

// Credential is a login kept with a project (database, admin panel, FTP…).
// Password only ever lives inside the encrypted vault.
type Credential struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Comments string `json:"comments,omitempty"`
}

func (c Credential) empty() bool {
	return c.Name == "" && c.Username == "" && c.Password == "" && c.Comments == ""
}

// Project lives on at most one host (HostID empty for none); a host can
// have many projects.
type Project struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	HostID      string       `json:"hostId,omitempty"`
	Location    string       `json:"location,omitempty"`
	Credentials []Credential `json:"credentials,omitempty"`
	Extra       string       `json:"extra,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// Normalize trims the single-line fields and drops blank credentials.
func (p *Project) Normalize() {
	p.Name = strings.TrimSpace(p.Name)
	p.HostID = strings.TrimSpace(p.HostID)
	p.Location = strings.TrimSpace(p.Location)
	creds := make([]Credential, 0, len(p.Credentials))
	for _, c := range p.Credentials {
		c.Name = strings.TrimSpace(c.Name)
		c.Username = strings.TrimSpace(c.Username)
		if !c.empty() {
			creds = append(creds, c)
		}
	}
	p.Credentials = creds
}

func (p Project) Validate() error {
	if p.Name == "" {
		return errors.New("project name is required")
	}
	return nil
}
