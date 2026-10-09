// SPDX-FileCopyrightText: 2018 gabrie30 and the gabrie30/ghorg contributors
// SPDX-FileCopyrightText: 2025 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package scm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// fetchServerProjectReposParallel fetches remaining pages of Bitbucket Server repos concurrently
func (c Bitbucket) fetchServerProjectReposParallel(
	projectKey string,
	firstPageRepos []ServerRepository,
	totalSize, limit int,
) ([]Repo, error) {
	return c.fetchServerReposParallel(firstPageRepos, totalSize, limit, "repos for project "+projectKey,
		func(start int) string {
			return fmt.Sprintf("/rest/api/1.0/projects/%s/repos?start=%d&limit=%d", projectKey, start, limit)
		})
}

// fetchServerUserReposParallel fetches remaining pages of Bitbucket Server user repos concurrently
func (c Bitbucket) fetchServerUserReposParallel(
	_ string,
	firstPageRepos []ServerRepository,
	totalSize, limit int,
) ([]Repo, error) {
	return c.fetchServerReposParallel(firstPageRepos, totalSize, limit, "user repos",
		func(start int) string {
			return fmt.Sprintf("/rest/api/1.0/repos?start=%d&limit=%d", start, limit)
		})
}

// fetchServerReposParallel filters the first page, fetches pages 2..n of a
// Bitbucket Server listing concurrently, and appends them in page order.
// pagePath returns the API path (after the server URL) for the page starting
// at start; what names the listing in errors.
func (c Bitbucket) fetchServerReposParallel(
	firstPageRepos []ServerRepository,
	totalSize, limit int,
	what string,
	pagePath func(start int) string,
) ([]Repo, error) {
	// Calculate total number of pages
	totalPages := (totalSize + limit - 1) / limit // Ceiling division

	// Filter first page
	repoData := make([]Repo, 0, totalSize)
	repoData = append(repoData, c.filterServerRepos(firstPageRepos)...)

	// If only one page, return immediately
	if totalPages <= 1 {
		return repoData, nil
	}

	// Channel to collect results from parallel fetches
	type pageResult struct {
		repos []ServerRepository
		err   error
		page  int
	}
	resultChan := make(chan pageResult, totalPages-1)

	// WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Fetch remaining pages in parallel
	for page := 2; page <= totalPages; page++ {
		wg.Add(1)
		go func(pageNum int) {
			defer wg.Done()
			apiURL := strings.TrimSuffix(c.serverURL, "/") + pagePath((pageNum-1)*limit)
			repos, err := c.fetchServerReposPage(apiURL, what)
			resultChan <- pageResult{repos: repos, err: err, page: pageNum}
		}(page)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results, organized by page number for consistent ordering
	pageResults := make(map[int][]ServerRepository, totalPages-1)
	for result := range resultChan {
		if result.err != nil {
			return nil, result.err
		}
		pageResults[result.page] = result.repos
	}

	// Append results in page order to maintain consistency
	for page := 2; page <= totalPages; page++ {
		if repos, ok := pageResults[page]; ok {
			repoData = append(repoData, c.filterServerRepos(repos)...)
		}
	}

	return repoData, nil
}

// fetchServerReposPage fetches one page of a Bitbucket Server repository
// listing.
func (c Bitbucket) fetchServerReposPage(apiURL, what string) ([]ServerRepository, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.username, c.password)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	body, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s (could not read response body: %w)", what, err)
		}
		return nil, fmt.Errorf("failed to fetch %s: %s", what, string(body))
	}
	if err != nil {
		return nil, err
	}

	var projectResp ServerProjectResponse
	if err := json.Unmarshal(body, &projectResp); err != nil {
		return nil, err
	}
	return projectResp.Values, nil
}
