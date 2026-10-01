package model

import "testing"

func TestNormalizeKeepsOnlySelectedSecrets(t *testing.T) {
	h := Host{AuthMethod: AuthKey, Password: "pw", PrivateKey: "key", KeyPassphrase: "pp"}
	h.Normalize()
	if h.Password != "" || h.PrivateKey != "key" || h.KeyPassphrase != "pp" {
		t.Fatalf("key auth kept wrong secrets: %+v", h)
	}

	h = Host{AuthMethod: AuthPassword, Password: "pw", PrivateKey: "key", KeyPassphrase: "pp"}
	h.Normalize()
	if h.Password != "pw" || h.PrivateKey != "" || h.KeyPassphrase != "" {
		t.Fatalf("password auth kept wrong secrets: %+v", h)
	}
}

func TestNormalizeInfersLegacyMethod(t *testing.T) {
	h := Host{PrivateKey: "key"}
	h.Normalize()
	if h.AuthMethod != AuthKey || h.Port != 22 {
		t.Fatalf("got %+v", h)
	}
}

func TestValidateKeyRequired(t *testing.T) {
	h := Host{Name: "a", Address: "b", Port: 22, Username: "c", AuthMethod: AuthKey}
	if h.Validate() == nil {
		t.Fatal("key auth without key must fail")
	}
}

func TestProjectNormalizeDropsBlankCredentials(t *testing.T) {
	p := Project{Name: " web ", Credentials: []Credential{{Name: "  "}, {Name: "db", Password: "x"}}}
	p.Normalize()
	if p.Name != "web" || len(p.Credentials) != 1 || p.Credentials[0].Name != "db" {
		t.Fatalf("got %+v", p)
	}
	if (Project{}).Validate() == nil {
		t.Fatal("project without name must fail")
	}
}
