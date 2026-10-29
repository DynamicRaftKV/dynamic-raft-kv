package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DynamicRaftKV/dynamic-raft-kv/internal/api"
)

func run(args []string, out io.Writer) error {
	f := flag.NewFlagSet("raftctl", flag.ContinueOnError)
	endpoint := f.String("endpoint", "http://localhost:8001", "HTTP base URL (before subcommand)")
	timeout := f.Duration("timeout", 5*time.Second, "request timeout")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	u, err := url.Parse(*endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("endpoint must be an HTTP(S) base URL without query or fragment")
	}
	var method, path string
	var payload any
	args = f.Args()
	if len(args) == 0 {
		return errors.New("usage: raftctl [flags] status | put KEY VALUE | get KEY | delete KEY | join ID GRPC_ADDRESS HTTP_ADDRESS | promote ID | remove ID")
	}
	switch args[0] {
	case "put", "get", "delete":
		want := 2
		if args[0] == "put" {
			want = 3
		}
		if len(args) != want || args[1] == "" || args[1] == "." || args[1] == ".." {
			return errors.New("usage: put KEY VALUE | get KEY | delete KEY (nonempty key, not . or ..)")
		}
		method = strings.ToUpper(args[0])
		path = "/kv/" + url.PathEscape(args[1])
		if args[0] == "put" {
			payload = api.PutRequest{Value: &args[2]}
		}

	case "status":
		if len(args) != 1 {
			return errors.New("usage: raftctl [flags] status")
		}
		method, path = http.MethodGet, "/cluster/status"
	case "join":
		if len(args) != 4 {
			return errors.New("usage: raftctl [flags] join ID GRPC_ADDRESS HTTP_ADDRESS")
		}
		method, path = http.MethodPost, "/cluster/join"
		payload = api.JoinRequest{ID: args[1], GRPCAddress: args[2], HTTPAddress: args[3]}
	case "remove", "promote":
		if len(args) != 2 {
			return errors.New("usage: raftctl [flags] remove ID | promote ID")
		}
		method, path = http.MethodPost, "/cluster/"+args[0]
		payload = api.RemoveRequest{ID: args[1]}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, strings.TrimRight(*endpoint, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: *timeout, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many leader redirects")
		}
		if next.Response == nil || next.Response.StatusCode != http.StatusTemporaryRedirect {
			return http.ErrUseLastResponse
		}
		if next.URL.Scheme != "http" && next.URL.Scheme != "https" {
			return errors.New("invalid leader URL")
		}
		if via[0].URL.Scheme == "https" && next.URL.Scheme != "https" {
			return errors.New("refusing HTTPS downgrade")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}
