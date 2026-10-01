package model

import (
	"errors"
	"strings"
)

// Group is a named, ordered section of hosts. Hosts reference it by ID.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (g *Group) Normalize() {
	g.Name = strings.TrimSpace(g.Name)
}

func (g Group) Validate() error {
	if g.Name == "" {
		return errors.New("group name is required")
	}
	return nil
}
