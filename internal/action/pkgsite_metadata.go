package action

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mishankov/updtr/internal/core"
)

const pkgsiteBaseURL = "https://pkg.go.dev"

type ModuleMetadataClient interface {
	Lookup(context.Context, string) (core.ModuleMetadata, error)
}

type pkgsiteMetadataClient struct {
	baseURL string
	client  *http.Client
}

func newPkgsiteMetadataClient(client *http.Client) ModuleMetadataClient {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return pkgsiteMetadataClient{baseURL: pkgsiteBaseURL, client: client}
}

func enrichPullRequestMetadata(ctx context.Context, result core.RunResult, client ModuleMetadataClient) core.RunResult {
	if client == nil {
		return result
	}

	cache := map[string]core.ModuleMetadata{}
	for targetIndex := range result.Targets {
		for updateIndex := range result.Targets[targetIndex].Applied {
			update := &result.Targets[targetIndex].Applied[updateIndex]
			if update.ModulePath == "" {
				continue
			}
			if metadata, ok := cache[update.ModulePath]; ok {
				update.Metadata = metadata
				continue
			}

			metadata, err := client.Lookup(ctx, update.ModulePath)
			if err != nil {
				cache[update.ModulePath] = core.ModuleMetadata{}
				continue
			}
			cache[update.ModulePath] = metadata
			update.Metadata = metadata
		}
	}
	return result
}

type pkgsiteModuleResponse struct {
	RepoURL string `json:"repoUrl"`
}

type pkgsitePackageResponse struct {
	Synopsis string `json:"synopsis"`
}

func (c pkgsiteMetadataClient) Lookup(ctx context.Context, modulePath string) (core.ModuleMetadata, error) {
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return core.ModuleMetadata{}, fmt.Errorf("empty module path")
	}

	metadata := core.ModuleMetadata{
		PackageURL: strings.TrimRight(c.baseURL, "/") + "/" + modulePath,
	}

	var module pkgsiteModuleResponse
	if err := c.getJSON(ctx, "/v1beta/module/"+escapedPath(modulePath), &module); err == nil {
		metadata.RepositoryURL = module.RepoURL
	}

	var pkg pkgsitePackageResponse
	if err := c.getJSON(ctx, "/v1beta/package/"+escapedPath(modulePath), &pkg); err == nil {
		metadata.Synopsis = strings.TrimSpace(pkg.Synopsis)
	}

	if metadata.RepositoryURL == "" && metadata.Synopsis == "" {
		return core.ModuleMetadata{}, fmt.Errorf("no pkg.go.dev metadata for %s", modulePath)
	}
	return metadata, nil
}

func (c pkgsiteMetadataClient) getJSON(ctx context.Context, path string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.baseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("pkg.go.dev returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func escapedPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
