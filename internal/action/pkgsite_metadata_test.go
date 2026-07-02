package action

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
)

func TestPkgsiteMetadataClientLookupCombinesModuleAndPackageData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1beta/module/github.com/example/mod":
			_, _ = w.Write([]byte(`{"repoUrl":"https://github.com/example/mod"}`))
		case "/v1beta/package/github.com/example/mod":
			_, _ = w.Write([]byte(`{"synopsis":"Package mod does useful things."}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := pkgsiteMetadataClient{baseURL: server.URL, client: server.Client()}
	metadata, err := client.Lookup(context.Background(), "github.com/example/mod")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.PackageURL != server.URL+"/github.com/example/mod" {
		t.Fatalf("PackageURL = %q", metadata.PackageURL)
	}
	if metadata.RepositoryURL != "https://github.com/example/mod" {
		t.Fatalf("RepositoryURL = %q", metadata.RepositoryURL)
	}
	if metadata.Synopsis != "Package mod does useful things." {
		t.Fatalf("Synopsis = %q", metadata.Synopsis)
	}
}

func TestEnrichPullRequestMetadataIsBestEffortAndCached(t *testing.T) {
	client := &fakeMetadataClient{
		results: map[string]core.ModuleMetadata{
			"github.com/example/mod": {
				PackageURL:    "https://pkg.go.dev/github.com/example/mod",
				RepositoryURL: "https://github.com/example/mod",
				Synopsis:      "Package mod does useful things.",
			},
		},
		errors: map[string]error{
			"github.com/example/private": errors.New("not indexed"),
		},
	}

	result := core.RunResult{Targets: []core.TargetResult{{
		Target: config.Target{Name: "app", NormalizedPath: "."},
		Applied: []core.AppliedUpdate{
			{ModulePath: "github.com/example/mod"},
			{ModulePath: "github.com/example/mod"},
			{ModulePath: "github.com/example/private"},
		},
	}}}

	enriched := enrichPullRequestMetadata(context.Background(), result, client)
	if got := client.calls["github.com/example/mod"]; got != 1 {
		t.Fatalf("metadata lookups for public module = %d, want 1", got)
	}
	if got := client.calls["github.com/example/private"]; got != 1 {
		t.Fatalf("metadata lookups for private module = %d, want 1", got)
	}
	if enriched.Targets[0].Applied[0].Metadata.Synopsis == "" {
		t.Fatalf("first public update was not enriched")
	}
	if enriched.Targets[0].Applied[1].Metadata.Synopsis == "" {
		t.Fatalf("second public update did not reuse cached metadata")
	}
	if enriched.Targets[0].Applied[2].Metadata.PackageURL != "" {
		t.Fatalf("failed metadata lookup should remain empty")
	}
}

type fakeMetadataClient struct {
	results map[string]core.ModuleMetadata
	errors  map[string]error
	calls   map[string]int
}

func (c *fakeMetadataClient) Lookup(_ context.Context, modulePath string) (core.ModuleMetadata, error) {
	if c.calls == nil {
		c.calls = map[string]int{}
	}
	c.calls[modulePath]++
	if err := c.errors[modulePath]; err != nil {
		return core.ModuleMetadata{}, err
	}
	return c.results[modulePath], nil
}
