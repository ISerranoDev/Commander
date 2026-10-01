package vault

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

func TestProjectsCRUD(t *testing.T) {
	v := newTestVault(t)
	h, _ := v.PutHost(sampleHost())
	if _, err := v.PutProject(model.Project{Name: "x", HostID: "ghost"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for unknown host, got %v", err)
	}
	p, err := v.PutProject(model.Project{
		Name: " shop ", HostID: h.ID, Location: "/var/www/shop",
		Credentials: []model.Credential{{Name: "db", Username: "app", Password: "dbpass"}, {}},
		Extra:       "notes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" || p.Name != "shop" || len(p.Credentials) != 1 || p.Credentials[0].ID == "" {
		t.Fatalf("unexpected project: %+v", p)
	}

	v.Lock()
	if err := v.Unlock(pw); err != nil {
		t.Fatal(err)
	}
	got, err := v.Project(p.ID)
	if err != nil || got.Credentials[0].Password != "dbpass" || got.Credentials[0].ID != p.Credentials[0].ID {
		t.Fatalf("got %+v, %v", got, err)
	}

	// Deleting the host keeps the project, without host.
	if err := v.DeleteHost(h.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := v.Project(p.ID); got.HostID != "" {
		t.Fatalf("project still points at deleted host: %+v", got)
	}
	if err := v.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if projects, _ := v.Projects(); len(projects) != 0 {
		t.Fatalf("project not deleted: %+v", projects)
	}
}

func TestExportImportKeepsProjects(t *testing.T) {
	src := newTestVault(t)
	h, _ := src.PutHost(sampleHost())
	p, _ := src.PutProject(model.Project{Name: "shop", HostID: h.ID, Credentials: []model.Credential{{Name: "db", Password: "dbpass"}}})
	if _, err := src.PutProject(model.Project{Name: "loose"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.commander")
	if err := src.Export(path, "export-pass"); err != nil {
		t.Fatal(err)
	}

	dst := newTestVault(t)
	n, err := dst.Import(path, "export-pass")
	if err != nil || n.Hosts != 1 || n.Projects != 2 {
		t.Fatalf("n=%+v err=%v", n, err)
	}
	if _, err := dst.Import(path, "export-pass"); err != nil {
		t.Fatal(err)
	}
	projects, _ := dst.Projects()
	if len(projects) != 2 {
		t.Fatalf("projects duplicated or lost: %+v", projects)
	}
	got, _ := dst.Project(p.ID)
	if got.HostID != h.ID || got.Credentials[0].Password != "dbpass" {
		t.Fatalf("got %+v", got)
	}
}

func TestProjectsSurviveHostAndGroupChanges(t *testing.T) {
	v := newTestVault(t)
	h, _ := v.PutHost(sampleHost())
	if _, err := v.PutProject(model.Project{Name: "shop", HostID: h.ID}); err != nil {
		t.Fatal(err)
	}
	g, _ := v.PutGroup(model.Group{Name: "g"})
	if err := v.ArrangeHosts([]Placement{{h.ID, g.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := v.TrustHost("10.0.0.1:22", "ssh-ed25519 AAAA"); err != nil {
		t.Fatal(err)
	}
	if projects, _ := v.Projects(); len(projects) != 1 || projects[0].HostID != h.ID {
		t.Fatalf("projects lost: %+v", projects)
	}
}
