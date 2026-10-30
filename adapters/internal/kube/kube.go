// Package kube is the small Kubernetes API client the built-in adapters use
// to read custom resources. It finds credentials the way kubectl does and
// reads lists through httpx.
package kube

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"fendlemark.app/fendlemark/adapters/internal/httpx"
)

// ErrNoConfig means no kubeconfig and no in-cluster service account was found.
var ErrNoConfig = errors.New("kube: no kubeconfig and not running in a cluster")

// Where a pod's service account credentials are mounted. Variables so tests
// can point them elsewhere.
var (
	saTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	saCAFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Client reads from one Kubernetes API server.
type Client struct {
	c         *httpx.Client
	tokenFile string
}

// Load finds credentials in this order: the first file in $KUBECONFIG that
// exists, or ~/.kube/config when $KUBECONFIG is unset; then the pod's own
// service account. It uses the kubeconfig's current context.
func Load(timeout time.Duration) (*Client, error) { return LoadPath("", timeout) }

// LoadPath is Load with an explicit kubeconfig, which must exist. An empty
// path means the standard search.
func LoadPath(path string, timeout time.Duration) (*Client, error) {
	if path != "" {
		return fromKubeconfig(path, timeout)
	}
	for _, p := range kubeconfigPaths() {
		if _, err := os.Stat(p); err == nil {
			return fromKubeconfig(p, timeout)
		}
	}
	return inCluster(timeout)
}

func kubeconfigPaths() []string {
	if v := os.Getenv("KUBECONFIG"); v != "" {
		return filepath.SplitList(v)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return []string{filepath.Join(home, ".kube", "config")}
	}
	return nil
}

func inCluster(timeout time.Duration) (*Client, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, ErrNoConfig
	}
	if _, err := os.Stat(saTokenFile); err != nil {
		return nil, ErrNoConfig
	}
	ca, err := os.ReadFile(saCAFile)
	if err != nil {
		return nil, fmt.Errorf("kube: in-cluster CA: %w", err)
	}
	tlsCfg, err := tlsConfig(ca, nil, nil, false)
	if err != nil {
		return nil, err
	}
	c, err := newClient("https://"+net.JoinHostPort(host, port), timeout, tlsCfg)
	if err != nil {
		return nil, err
	}
	return &Client{c: c, tokenFile: saTokenFile}, nil
}

type kubeconfig struct {
	CurrentContext string `yaml:"current-context"`
	Contexts       []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Clusters []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server   string `yaml:"server"`
			CAData   string `yaml:"certificate-authority-data"`
			CAFile   string `yaml:"certificate-authority"`
			Insecure bool   `yaml:"insecure-skip-tls-verify"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token     string `yaml:"token"`
			TokenFile string `yaml:"tokenFile"`
			CertData  string `yaml:"client-certificate-data"`
			CertFile  string `yaml:"client-certificate"`
			KeyData   string `yaml:"client-key-data"`
			KeyFile   string `yaml:"client-key"`
			Username  string `yaml:"username"`
			Password  string `yaml:"password"`
			Exec      *struct {
				APIVersion string   `yaml:"apiVersion"`
				Command    string   `yaml:"command"`
				Args       []string `yaml:"args"`
				Env        []struct {
					Name  string `yaml:"name"`
					Value string `yaml:"value"`
				} `yaml:"env"`
			} `yaml:"exec"`
		} `yaml:"user"`
	} `yaml:"users"`
}

func fromKubeconfig(path string, timeout time.Duration) (*Client, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kube: %w", err)
	}
	var kc kubeconfig
	if err := yaml.Unmarshal(raw, &kc); err != nil {
		return nil, fmt.Errorf("kube: %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	rel := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(dir, p)
	}
	var clusterName, userName string
	for _, c := range kc.Contexts {
		if c.Name == kc.CurrentContext {
			clusterName, userName = c.Context.Cluster, c.Context.User
		}
	}
	if clusterName == "" {
		return nil, fmt.Errorf("kube: %s: current context %q not found", path, kc.CurrentContext)
	}
	var server string
	var ca []byte
	var insecure bool
	for _, c := range kc.Clusters {
		if c.Name != clusterName {
			continue
		}
		server, insecure = c.Cluster.Server, c.Cluster.Insecure
		if ca, err = dataOrFile(c.Cluster.CAData, rel(c.Cluster.CAFile)); err != nil {
			return nil, fmt.Errorf("kube: certificate authority: %w", err)
		}
	}
	if server == "" {
		return nil, fmt.Errorf("kube: %s: cluster %q has no server", path, clusterName)
	}
	cl := &Client{}
	var cert, key []byte
	var bearer, user, password string
	for _, u := range kc.Users {
		if u.Name != userName {
			continue
		}
		bearer, user, password = u.User.Token, u.User.Username, u.User.Password
		cl.tokenFile = rel(u.User.TokenFile)
		if cert, err = dataOrFile(u.User.CertData, rel(u.User.CertFile)); err != nil {
			return nil, fmt.Errorf("kube: client certificate: %w", err)
		}
		if key, err = dataOrFile(u.User.KeyData, rel(u.User.KeyFile)); err != nil {
			return nil, fmt.Errorf("kube: client key: %w", err)
		}
		if e := u.User.Exec; e != nil {
			env := os.Environ()
			for _, kv := range e.Env {
				env = append(env, kv.Name+"="+kv.Value)
			}
			cred, err := runExec(e.APIVersion, e.Command, e.Args, env, timeout)
			if err != nil {
				return nil, err
			}
			bearer = cred.Token
			if cred.ClientCertificateData != "" {
				cert, key = []byte(cred.ClientCertificateData), []byte(cred.ClientKeyData)
			}
		}
	}
	tlsCfg, err := tlsConfig(ca, cert, key, insecure)
	if err != nil {
		return nil, err
	}
	if cl.c, err = newClient(server, timeout, tlsCfg); err != nil {
		return nil, err
	}
	cl.c.Bearer, cl.c.User, cl.c.Password = bearer, user, password
	return cl, nil
}

type execCredential struct {
	Token                 string `json:"token"`
	ClientCertificateData string `json:"clientCertificateData"`
	ClientKeyData         string `json:"clientKeyData"`
}

// runExec runs a client-go credential plugin once, as kubectl does, and
// returns its credential. Credentials are read when the adapter connects.
func runExec(apiVersion, command string, args, env []string, timeout time.Duration) (execCredential, error) {
	if apiVersion == "" {
		apiVersion = "client.authentication.k8s.io/v1"
	}
	info, _ := json.Marshal(map[string]any{"apiVersion": apiVersion, "kind": "ExecCredential", "spec": map[string]any{"interactive": false}})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(env, "KUBERNETES_EXEC_INFO="+string(info))
	out, err := cmd.Output()
	if err != nil {
		return execCredential{}, fmt.Errorf("kube: credential plugin %s: %w", command, err)
	}
	var resp struct {
		Status execCredential `json:"status"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return execCredential{}, fmt.Errorf("kube: credential plugin %s: %w", command, err)
	}
	return resp.Status, nil
}

func dataOrFile(data, file string) ([]byte, error) {
	if data != "" {
		return base64.StdEncoding.DecodeString(data)
	}
	if file != "" {
		return os.ReadFile(file)
	}
	return nil, nil
}

func tlsConfig(ca, cert, key []byte, insecure bool) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: insecure}
	if len(ca) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, errors.New("kube: certificate authority holds no PEM certificates")
		}
		cfg.RootCAs = pool
	}
	if len(cert) > 0 {
		pair, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("kube: client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

func newClient(server string, timeout time.Duration, tlsCfg *tls.Config) (*httpx.Client, error) {
	c, err := httpx.New(server, timeout)
	if err != nil {
		return nil, fmt.Errorf("kube: %w", err)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = tlsCfg
	c.HTTP = &http.Client{Timeout: c.Timeout, Transport: tr}
	return c, nil
}

// Bearer is the bearer token the credentials carry, if any. Argo Server in
// client mode accepts the same token as the Kubernetes API.
func (k *Client) Bearer() string {
	k.refresh()
	return k.c.Bearer
}

// refresh rereads a token file, since projected service account tokens rotate.
func (k *Client) refresh() {
	if k.tokenFile != "" {
		if b, err := os.ReadFile(k.tokenFile); err == nil {
			k.c.Bearer = strings.TrimSpace(string(b))
		}
	}
}

func resourcePath(group, version, namespace, resource string) string {
	path := "/apis/" + group + "/" + version + "/"
	if namespace != "" {
		path += "namespaces/" + url.PathEscape(namespace) + "/"
	}
	return path + resource
}

// List reads a list of custom resources, all namespaces when namespace is
// empty, into out; query carries options such as limit and continue. A
// resource the cluster does not serve, because its CRD is not installed, is
// reported with ok false and no error.
func (k *Client) List(ctx context.Context, group, version, namespace, resource string, query url.Values, out any) (ok bool, err error) {
	k.refresh()
	if err := k.c.GetJSON(ctx, resourcePath(group, version, namespace, resource), query, out); err != nil {
		var he *httpx.Error
		if errors.As(err, &he) && he.Status == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// WatchEvent is one event from a Kubernetes watch: ADDED, MODIFIED, DELETED,
// BOOKMARK, or ERROR, whose object is then a Status.
type WatchEvent struct {
	Type   string          `json:"type"`
	Object json.RawMessage `json:"object"`
}

// ErrExpired means the resource version a watch started from is too old
// (HTTP 410); start again from an empty one.
var ErrExpired = errors.New("kube: resource version expired")

// Watch streams changes to a resource, calling fn for each event, until the
// server closes the stream, ctx is done, or fn returns an error. With an
// empty resourceVersion the stream starts with an ADDED event for every
// object that exists. The server closes a stream after timeoutSeconds, so
// callers reconnect from the last resource version they saw.
func (k *Client) Watch(ctx context.Context, group, version, namespace, resource, resourceVersion string, fn func(WatchEvent) error) error {
	k.refresh()
	q := url.Values{"watch": {"true"}, "allowWatchBookmarks": {"true"}, "timeoutSeconds": {"300"}}
	if resourceVersion != "" {
		q.Set("resourceVersion", resourceVersion)
	}
	u := k.c.Base + resourcePath(group, version, namespace, resource) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fendlemark/0.9.13")
	if k.c.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+k.c.Bearer)
	} else if k.c.User != "" {
		req.SetBasicAuth(k.c.User, k.c.Password)
	}
	// The stream outlives the client's request timeout, so use its transport
	// without one; the server's timeoutSeconds bounds it instead.
	resp, err := (&http.Client{Transport: k.c.HTTP.Transport}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		return ErrExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &httpx.Error{Status: resp.StatusCode, Body: string(b), URL: u}
	}
	dec := json.NewDecoder(resp.Body)
	for {
		var ev WatchEvent
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) || ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if ev.Type == "ERROR" {
			var st struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			json.Unmarshal(ev.Object, &st)
			if st.Code == http.StatusGone {
				return ErrExpired
			}
			return fmt.Errorf("kube: watch %s: %s", resource, st.Message)
		}
		if err := fn(ev); err != nil {
			return err
		}
	}
}
