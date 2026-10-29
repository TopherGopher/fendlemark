package kube

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func cluster(t *testing.T, wantToken string) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+wantToken {
			http.Error(w, "unauthorised", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/apis/kubeflow.org/v1/namespaces/team-ml/notebooks":
			w.Write([]byte(`{"items":[{"metadata":{"name":"nb"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func caPEM(srv *httptest.Server) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
}

func noCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
}

func TestNoConfig(t *testing.T) {
	noCluster(t)
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "absent"))
	if _, err := Load(time.Second); !errors.Is(err, ErrNoConfig) {
		t.Fatalf("want ErrNoConfig, got %v", err)
	}
}

func TestKubeconfig(t *testing.T) {
	noCluster(t)
	srv := cluster(t, "s3cret")
	dir := t.TempDir()
	kc := `apiVersion: v1
kind: Config
current-context: lab
contexts:
- name: lab
  context: {cluster: lab, user: reader}
- name: other
  context: {cluster: elsewhere, user: nobody}
clusters:
- name: lab
  cluster:
    server: ` + srv.URL + `
    certificate-authority-data: ` + base64.StdEncoding.EncodeToString(caPEM(srv)) + `
users:
- name: reader
  user: {token: s3cret}
`
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(kc), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", filepath.Join(dir, "absent")+string(filepath.ListSeparator)+path)
	c, err := Load(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var nbs struct {
		Items []struct {
			Metadata struct{ Name string } `json:"metadata"`
		} `json:"items"`
	}
	ok, err := c.List(context.Background(), "kubeflow.org", "v1", "team-ml", "notebooks", "", &nbs)
	if err != nil || !ok || len(nbs.Items) != 1 || nbs.Items[0].Metadata.Name != "nb" {
		t.Fatalf("list: ok=%v err=%v %+v", ok, err, nbs)
	}
	ok, err = c.List(context.Background(), "kubeflow.org", "v1beta1", "team-ml", "experiments", "", &nbs)
	if err != nil || ok {
		t.Fatalf("a resource the cluster does not serve should be ok=false with no error, got ok=%v err=%v", ok, err)
	}
}

func TestInCluster(t *testing.T) {
	srv := cluster(t, "rotated")
	dir := t.TempDir()
	tok, ca := filepath.Join(dir, "token"), filepath.Join(dir, "ca.crt")
	os.WriteFile(tok, []byte("first\n"), 0o600)
	os.WriteFile(ca, caPEM(srv), 0o600)
	saTokenFile, saCAFile = tok, ca
	t.Cleanup(func() {
		saTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
		saCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	})
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	t.Setenv("KUBECONFIG", filepath.Join(dir, "absent"))
	t.Setenv("KUBERNETES_SERVICE_HOST", host)
	t.Setenv("KUBERNETES_SERVICE_PORT", port)
	c, err := Load(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if _, err := c.List(context.Background(), "kubeflow.org", "v1", "team-ml", "notebooks", "", &out); err == nil {
		t.Fatal("the first token should be rejected")
	}
	// The kubelet rotates the projected token; the next request uses the new one.
	os.WriteFile(tok, []byte("rotated\n"), 0o600)
	if ok, err := c.List(context.Background(), "kubeflow.org", "v1", "team-ml", "notebooks", "", &out); err != nil || !ok {
		t.Fatalf("rotated token: ok=%v err=%v", ok, err)
	}
}
