package app

import (
	"time"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

// CredentialSummary is a project credential without its password.
type CredentialSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	Comments    string `json:"comments"`
	HasPassword bool   `json:"hasPassword"`
}

// ProjectSummary is what the UI gets: credential passwords stay in the
// vault until revealed one by one.
type ProjectSummary struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	HostID      string              `json:"hostId"`
	Location    string              `json:"location"`
	Credentials []CredentialSummary `json:"credentials"`
	Extra       string              `json:"extra"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

func summarizeProject(p model.Project) ProjectSummary {
	creds := make([]CredentialSummary, len(p.Credentials))
	for i, c := range p.Credentials {
		creds[i] = CredentialSummary{
			ID:          c.ID,
			Name:        c.Name,
			Username:    c.Username,
			Comments:    c.Comments,
			HasPassword: c.Password != "",
		}
	}
	return ProjectSummary{
		ID:          p.ID,
		Name:        p.Name,
		HostID:      p.HostID,
		Location:    p.Location,
		Credentials: creds,
		Extra:       p.Extra,
		UpdatedAt:   p.UpdatedAt,
	}
}

func (a *App) ListProjects() ([]ProjectSummary, error) {
	projects, err := a.vault.Projects()
	if err != nil {
		return nil, err
	}
	out := make([]ProjectSummary, len(projects))
	for i, p := range projects {
		out[i] = summarizeProject(p)
	}
	return out, nil
}

func (a *App) GetProject(id string) (ProjectSummary, error) {
	p, err := a.vault.Project(id)
	if err != nil {
		return ProjectSummary{}, err
	}
	return summarizeProject(p), nil
}

// CredentialInput comes from the project form. An empty password on an
// existing credential (same ID) keeps the stored one.
type CredentialInput struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Comments string `json:"comments"`
}

type ProjectInput struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	HostID      string            `json:"hostId"`
	Location    string            `json:"location"`
	Credentials []CredentialInput `json:"credentials"`
	Extra       string            `json:"extra"`
}

func (a *App) SaveProject(in ProjectInput) (ProjectSummary, error) {
	stored := map[string]string{}
	if in.ID != "" {
		current, err := a.vault.Project(in.ID)
		if err != nil {
			return ProjectSummary{}, err
		}
		for _, c := range current.Credentials {
			stored[c.ID] = c.Password
		}
	}
	p := model.Project{
		ID:       in.ID,
		Name:     in.Name,
		HostID:   in.HostID,
		Location: in.Location,
		Extra:    in.Extra,
	}
	for _, c := range in.Credentials {
		password := c.Password
		if password == "" && c.ID != "" {
			password = stored[c.ID]
		}
		p.Credentials = append(p.Credentials, model.Credential{
			ID:       c.ID,
			Name:     c.Name,
			Username: c.Username,
			Password: password,
			Comments: c.Comments,
		})
	}
	saved, err := a.vault.PutProject(p)
	if err != nil {
		return ProjectSummary{}, err
	}
	return summarizeProject(saved), nil
}

func (a *App) DeleteProject(id string) error {
	return a.vault.DeleteProject(id)
}

// RevealCredential returns the stored password of a project credential so
// the UI can show or copy it on demand.
func (a *App) RevealCredential(projectID, credentialID string) (string, error) {
	p, err := a.vault.Project(projectID)
	if err != nil {
		return "", err
	}
	for _, c := range p.Credentials {
		if c.ID == credentialID {
			return c.Password, nil
		}
	}
	return "", errUnknownSecret
}
