package vault

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ISerranoDev/WailsCommander/internal/model"
)

func hostNamed(name string) model.Host {
	h := sampleHost()
	h.Name = name
	return h
}

func names(t *testing.T, v *Vault) []string {
	t.Helper()
	hosts, err := v.Hosts()
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(hosts))
	for i, h := range hosts {
		out[i] = h.Name + "@" + h.GroupID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeV1Export writes an export exactly as versions without groups did:
// version 1, no "groups" key, no "groupId" on hosts.
func writeV1Export(t *testing.T, path, passphrase string, hosts []model.Host) {
	t.Helper()
	key, err := newSealed([]byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	defer key.wipe()
	in := make([]map[string]any, len(hosts))
	for i, h := range hosts {
		in[i] = map[string]any{
			"id": h.ID, "name": h.Name, "address": h.Address, "port": 22,
			"username": h.Username, "authMethod": "password", "password": h.Password,
		}
	}
	raw, err := json.Marshal(map[string]any{"v": 1, "hosts": in})
	if err != nil {
		t.Fatal(err)
	}
	blob, err := key.encrypt(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestImportV1ExportWithoutGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.commander")
	writeV1Export(t, path, "export-pass", []model.Host{hostNamed("zeta"), hostNamed("Alpha")})

	v := newTestVault(t)
	g, _ := v.PutGroup(model.Group{Name: "prod"})
	if _, err := v.PutHost(func() model.Host { h := hostNamed("mine"); h.GroupID = g.ID; return h }()); err != nil {
		t.Fatal(err)
	}
	n, err := v.Import(path, "export-pass")
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	// Old files keep their former (alphabetical) order and land ungrouped.
	want := []string{"mine@" + g.ID, "Alpha@", "zeta@"}
	if got := names(t, v); !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if groups, _ := v.Groups(); len(groups) != 1 {
		t.Fatalf("existing groups must survive: %+v", groups)
	}
}

func TestExportImportKeepsGroups(t *testing.T) {
	src := newTestVault(t)
	web, _ := src.PutGroup(model.Group{Name: "web"})
	db, _ := src.PutGroup(model.Group{Name: "db"})
	a, _ := src.PutHost(hostNamed("a"))
	b, _ := src.PutHost(hostNamed("b"))
	c, _ := src.PutHost(hostNamed("c"))
	if err := src.ArrangeHosts([]Placement{{c.ID, db.ID}, {a.ID, web.ID}, {b.ID, ""}}); err != nil {
		t.Fatal(err)
	}
	if err := src.ReorderGroups([]string{db.ID, web.ID}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.commander")
	if err := src.Export(path, "export-pass"); err != nil {
		t.Fatal(err)
	}

	dst := newTestVault(t)
	if _, err := dst.Import(path, "export-pass"); err != nil {
		t.Fatal(err)
	}
	groups, _ := dst.Groups()
	if len(groups) != 2 || groups[0].Name != "db" || groups[1].Name != "web" {
		t.Fatalf("groups: %+v", groups)
	}
	want := []string{"c@" + db.ID, "a@" + web.ID, "b@"}
	if got := names(t, dst); !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// Re-import does not duplicate groups.
	if _, err := dst.Import(path, "export-pass"); err != nil {
		t.Fatal(err)
	}
	if groups, _ := dst.Groups(); len(groups) != 2 {
		t.Fatalf("groups duplicated: %+v", groups)
	}
}

func TestGroupRules(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.PutGroup(model.Group{Name: "  "}); err == nil {
		t.Fatal("empty name must fail")
	}
	if _, err := v.PutGroup(model.Group{ID: "nope", Name: "x"}); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("want ErrGroupNotFound, got %v", err)
	}
	h := hostNamed("a")
	h.GroupID = "nope"
	if _, err := v.PutHost(h); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("want ErrGroupNotFound, got %v", err)
	}

	g, _ := v.PutGroup(model.Group{Name: " web "})
	if g.Name != "web" {
		t.Fatalf("name not trimmed: %q", g.Name)
	}
	g, _ = v.PutGroup(model.Group{ID: g.ID, Name: "front"})
	h.GroupID = g.ID
	saved, err := v.PutHost(h)
	if err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if err := v.Unlock(pw); err != nil {
		t.Fatal(err)
	}
	if groups, _ := v.Groups(); len(groups) != 1 || groups[0].Name != "front" {
		t.Fatalf("groups after unlock: %+v", groups)
	}
	// Deleting a group keeps its hosts, ungrouped.
	if err := v.DeleteGroup(g.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := v.Host(saved.ID); got.GroupID != "" {
		t.Fatalf("host still in deleted group: %+v", got)
	}
}

func TestChangingGroupMovesHostLast(t *testing.T) {
	v := newTestVault(t)
	g, _ := v.PutGroup(model.Group{Name: "g"})
	a, _ := v.PutHost(hostNamed("a"))
	if _, err := v.PutHost(hostNamed("b")); err != nil {
		t.Fatal(err)
	}
	a.GroupID = g.ID
	if _, err := v.PutHost(a); err != nil {
		t.Fatal(err)
	}
	want := []string{"b@", "a@" + g.ID}
	if got := names(t, v); !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestArrangeIgnoresUnknownAndKeepsMissing(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.PutHost(hostNamed("a")); err != nil {
		t.Fatal(err)
	}
	b, _ := v.PutHost(hostNamed("b"))
	c, _ := v.PutHost(hostNamed("c"))
	if err := v.ArrangeHosts([]Placement{{"ghost", ""}, {c.ID, "ghost-group"}, {b.ID, ""}, {c.ID, ""}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"c@", "b@", "a@"}
	if got := names(t, v); !equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// Vaults created before groups existed have no "groups" key; the UI must
// get an empty list, not null.
func TestGroupsNeverNull(t *testing.T) {
	v := newTestVault(t)
	path := v.Path()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeV1Export(t, path, pw, []model.Host{hostNamed("old")})
	if err := v.Unlock(pw); err != nil {
		t.Fatal(err)
	}
	groups, err := v.Groups()
	if err != nil || groups == nil {
		t.Fatalf("groups=%#v err=%v", groups, err)
	}
	raw, _ := json.Marshal(groups)
	if string(raw) != "[]" {
		t.Fatalf("got %s", raw)
	}
	if hosts, _ := v.Hosts(); len(hosts) != 1 {
		t.Fatalf("hosts lost: %+v", hosts)
	}
}
