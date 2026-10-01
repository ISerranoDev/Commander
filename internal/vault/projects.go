package vault

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

func (v *Vault) Projects() ([]model.Project, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.key == nil {
		return nil, ErrLocked
	}
	return append([]model.Project{}, v.data.Projects...), nil
}

func (v *Vault) Project(id string) (model.Project, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.key == nil {
		return model.Project{}, ErrLocked
	}
	i := v.projectIndex(id)
	if i < 0 {
		return model.Project{}, ErrProjectNotFound
	}
	return v.data.Projects[i], nil
}

// PutProject inserts p when its ID is empty, otherwise replaces the project
// with that ID. Its host, if set, must exist.
func (v *Vault) PutProject(p model.Project) (model.Project, error) {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return model.Project{}, err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return model.Project{}, ErrLocked
	}
	if p.HostID != "" && v.indexOf(p.HostID) < 0 {
		return model.Project{}, ErrNotFound
	}
	assignCredentialIDs(&p)

	next := slices.Clone(v.data.Projects)
	now := time.Now().UTC()
	p.UpdatedAt = now
	if p.ID == "" {
		p.ID = uuid.NewString()
		p.CreatedAt = now
		next = append(next, p)
	} else {
		i := v.projectIndex(p.ID)
		if i < 0 {
			return model.Project{}, ErrProjectNotFound
		}
		p.CreatedAt = next[i].CreatedAt
		next[i] = p
	}
	d := v.draft()
	d.Projects = next
	if err := v.commitData(d); err != nil {
		return model.Project{}, err
	}
	return p, nil
}

func (v *Vault) DeleteProject(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.key == nil {
		return ErrLocked
	}
	i := v.projectIndex(id)
	if i < 0 {
		return ErrProjectNotFound
	}
	d := v.draft()
	d.Projects = slices.Delete(slices.Clone(v.data.Projects), i, i+1)
	return v.commitData(d)
}

func (v *Vault) projectIndex(id string) int {
	return slices.IndexFunc(v.data.Projects, func(p model.Project) bool { return p.ID == id })
}

// assignCredentialIDs gives new (or clashing) credentials a fresh ID, so
// each one can be addressed to reveal or keep its password.
func assignCredentialIDs(p *model.Project) {
	creds := slices.Clone(p.Credentials)
	seen := map[string]bool{}
	for i := range creds {
		if creds[i].ID == "" || seen[creds[i].ID] {
			creds[i].ID = uuid.NewString()
		}
		seen[creds[i].ID] = true
	}
	p.Credentials = creds
}
