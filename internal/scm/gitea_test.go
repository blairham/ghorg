package scm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"code.gitea.io/sdk/gitea"
)

// mockGiteaRepository creates a mock Gitea repository for testing
func mockGiteaRepository(id int64, name string) *gitea.Repository {
	return &gitea.Repository{
		ID:            id,
		Name:          name,
		FullName:      fmt.Sprintf("test-org/%s", name),
		CloneURL:      fmt.Sprintf("https://gitea.example.com/test-org/%s.git", name),
		SSHURL:        fmt.Sprintf("git@gitea.example.com:test-org/%s.git", name),
		Private:       false,
		Fork:          false,
		Archived:      false,
		DefaultBranch: "main",
		Owner: &gitea.User{
			UserName: "test-org",
		},
	}
}

// setupGiteaTest creates a test server and Gitea client for testing
func setupGiteaTest() (client Gitea, mux *http.ServeMux, serverURL string, teardown func()) {
	// Create a test HTTP server
	mux = http.NewServeMux()

	// Mock the version endpoint that Gitea client calls during initialization
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"version": "1.18.0"})
	})

	// Mock the settings endpoint
	mux.HandleFunc("/api/v1/settings/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"max_response_items": 50,
		})
	})

	server := httptest.NewServer(mux)

	// Create Gitea client with custom base URL
	giteaClient, err := gitea.NewClient(server.URL)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Gitea client: %v", err))
	}

	client = Gitea{
		Client:  giteaClient,
		perPage: 10, // Small page size for testing pagination
	}

	return client, mux, server.URL, server.Close
}

func TestGitea_GetOrgRepos_SinglePage(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	// Mock API response for single page
	repos := make([]*gitea.Repository, 5)
	for i := 0; i < 5; i++ {
		repos[i] = mockGiteaRepository(int64(i+1), fmt.Sprintf("repo-%03d", i+1))
	}

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	// Set required environment variables
	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	if len(result) != 5 {
		t.Errorf("Expected 5 repositories, got %d", len(result))
	}

	// Verify repository data
	for i, repo := range result {
		expectedName := fmt.Sprintf("repo-%03d", i+1)
		if repo.Name != expectedName {
			t.Errorf("Expected repository name %s, got %s", expectedName, repo.Name)
		}
	}
}

func TestGitea_GetOrgRepos_MultiplePage_PaginationBugRegression(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	// This test specifically catches the pagination bug where perPage was undefined
	// and defaulted to 0, causing infinite loops or early termination

	pageRequests := 0
	totalRepos := 25 // More than perPage (10) to force pagination

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		pageRequests++

		// Parse page parameter
		page := 1
		if pageParam := r.URL.Query().Get("page"); pageParam != "" {
			if p, err := fmt.Sscanf(pageParam, "%d", &page); p != 1 || err != nil {
				page = 1
			}
		}

		// Calculate repositories for this page
		startIdx := (page - 1) * client.perPage
		endIdx := startIdx + client.perPage
		if endIdx > totalRepos {
			endIdx = totalRepos
		}

		// Handle pages beyond available data
		var repos []*gitea.Repository
		if startIdx < totalRepos {
			repos = make([]*gitea.Repository, 0, endIdx-startIdx)
			for i := startIdx; i < endIdx; i++ {
				repos = append(repos, mockGiteaRepository(int64(i+1), fmt.Sprintf("repo-%03d", i+1)))
			}
		} else {
			repos = []*gitea.Repository{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	// Set required environment variables
	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	// Verify all repositories were fetched across multiple pages
	if len(result) != totalRepos {
		t.Errorf("Expected %d repositories, got %d", totalRepos, len(result))
	}

	// Verify pagination occurred with parallel fetching
	// With parallel pagination, we fetch in batches of 10 pages at a time
	// So for 25 repos (3 pages), we'll request: page 1 + pages 2-11 (batch) = 11 requests total
	// This is expected - we make more requests but get better performance through concurrency
	expectedMinPages := 3  // At minimum we need 3 pages for 25 repos (10+10+5)
	expectedMaxPages := 11 // With batch size of 10, we'll request up to page 11
	if pageRequests < expectedMinPages || pageRequests > expectedMaxPages {
		t.Errorf("Expected between %d and %d page requests (parallel pagination), got %d",
			expectedMinPages, expectedMaxPages, pageRequests)
	}

	// Verify repository data continuity across pages
	for i, repo := range result {
		expectedName := fmt.Sprintf("repo-%03d", i+1)
		if repo.Name != expectedName {
			t.Errorf("Expected repository name %s, got %s", expectedName, repo.Name)
		}
	}
}

func TestGitea_GetOrgRepos_ExactPageBoundary(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	// Test the exact scenario that caused the original bug:
	// When repositories count is exactly divisible by perPage
	totalRepos := 50 // Exactly 5 pages of 10 repos each

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if pageParam := r.URL.Query().Get("page"); pageParam != "" {
			fmt.Sscanf(pageParam, "%d", &page)
		}

		startIdx := (page - 1) * client.perPage
		endIdx := startIdx + client.perPage
		if endIdx > totalRepos {
			endIdx = totalRepos
		}

		// Handle pages beyond available data
		var repos []*gitea.Repository
		if startIdx < totalRepos {
			repos = make([]*gitea.Repository, 0, endIdx-startIdx)
			for i := startIdx; i < endIdx; i++ {
				repos = append(repos, mockGiteaRepository(int64(i+1), fmt.Sprintf("repo-%03d", i+1)))
			}
		} else {
			repos = []*gitea.Repository{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	// This would fail with the original bug where perPage was undefined (defaulting to 0)
	// The comparison len(rps) < perPage would be len(rps) < 0, always false, causing infinite loop
	if len(result) != totalRepos {
		t.Errorf("Expected %d repositories, got %d - this indicates a pagination bug", totalRepos, len(result))
	}
}

func TestGitea_GetUserRepos_PaginationBugRegression(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	// Test user repository pagination - same bug affects GetUserRepos
	totalRepos := 15

	mux.HandleFunc("/api/v1/users/test-user/repos", func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if pageParam := r.URL.Query().Get("page"); pageParam != "" {
			fmt.Sscanf(pageParam, "%d", &page)
		}

		startIdx := (page - 1) * client.perPage
		endIdx := startIdx + client.perPage
		if endIdx > totalRepos {
			endIdx = totalRepos
		}

		// Handle pages beyond available data
		var repos []*gitea.Repository
		if startIdx < totalRepos {
			repos = make([]*gitea.Repository, 0, endIdx-startIdx)
			for i := startIdx; i < endIdx; i++ {
				repos = append(repos, mockGiteaRepository(int64(i+1), fmt.Sprintf("user-repo-%03d", i+1)))
			}
		} else {
			repos = []*gitea.Repository{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	result, err := client.GetUserRepos("test-user")
	if err != nil {
		t.Fatalf("GetUserRepos failed: %v", err)
	}

	if len(result) != totalRepos {
		t.Errorf("Expected %d repositories, got %d", totalRepos, len(result))
	}
}

func TestGitea_GetOrgRepos_EmptyResponse(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	mux.HandleFunc("/api/v1/orgs/empty-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gitea.Repository{})
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	result, err := client.GetOrgRepos("empty-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected 0 repositories for empty org, got %d", len(result))
	}
}

func TestGitea_GetOrgRepos_NotFound(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	mux.HandleFunc("/api/v1/orgs/nonexistent-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Not Found"))
	})

	_, err := client.GetOrgRepos("nonexistent-org")
	if err == nil {
		t.Fatal("Expected error for nonexistent org, got nil")
	}

	expectedError := `org "nonexistent-org" not found`
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGitea_AddTokenToCloneURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		token    string
		insecure string
		expected string
	}{
		{
			name:     "HTTPS URL adds token",
			url:      "https://gitea.example.com/org/repo.git",
			token:    "mytoken",
			expected: "https://mytoken@gitea.example.com/org/repo.git",
		},
		{
			name:     "HTTPS URL with different token",
			url:      "https://gitea.example.com/user/another-repo.git",
			token:    "abc123",
			expected: "https://abc123@gitea.example.com/user/another-repo.git",
		},
		{
			name:     "HTTP URL with insecure flag",
			url:      "http://gitea.local/org/repo.git",
			token:    "mytoken",
			insecure: "true",
			expected: "http://mytoken@gitea.local/org/repo.git",
		},
		{
			name:     "HTTPS URL with port",
			url:      "https://gitea.example.com:3000/org/repo.git",
			token:    "tok",
			expected: "https://tok@gitea.example.com:3000/org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.insecure != "" {
				os.Setenv("GHORG_INSECURE_GITEA_CLIENT", tt.insecure)
				defer os.Unsetenv("GHORG_INSECURE_GITEA_CLIENT")
			}

			g := Gitea{}
			result := g.addTokenToCloneURL(tt.url, tt.token)
			if result != tt.expected {
				t.Errorf("addTokenToCloneURL(%q, %q) = %q, want %q", tt.url, tt.token, result, tt.expected)
			}
		})
	}
}

func TestGitea_GetOrgRepos_SkipArchived(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	repos := []*gitea.Repository{
		{
			ID:            1,
			Name:          "active-repo",
			FullName:      "test-org/active-repo",
			CloneURL:      "https://gitea.example.com/test-org/active-repo.git",
			SSHURL:        "git@gitea.example.com:test-org/active-repo.git",
			Archived:      false,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
		{
			ID:            2,
			Name:          "archived-repo",
			FullName:      "test-org/archived-repo",
			CloneURL:      "https://gitea.example.com/test-org/archived-repo.git",
			SSHURL:        "git@gitea.example.com:test-org/archived-repo.git",
			Archived:      true,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
		{
			ID:            3,
			Name:          "another-active",
			FullName:      "test-org/another-active",
			CloneURL:      "https://gitea.example.com/test-org/another-active.git",
			SSHURL:        "git@gitea.example.com:test-org/another-active.git",
			Archived:      false,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
	}

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_SKIP_ARCHIVED", "true")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_SKIP_ARCHIVED")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 repositories (archived excluded), got %d", len(result))
	}

	for _, repo := range result {
		if repo.Name == "archived-repo" {
			t.Errorf("Archived repo should have been excluded, but found %q", repo.Name)
		}
	}
}

func TestGitea_GetOrgRepos_SkipForks(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	repos := []*gitea.Repository{
		{
			ID:            1,
			Name:          "original-repo",
			FullName:      "test-org/original-repo",
			CloneURL:      "https://gitea.example.com/test-org/original-repo.git",
			SSHURL:        "git@gitea.example.com:test-org/original-repo.git",
			Fork:          false,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
		{
			ID:            2,
			Name:          "forked-repo",
			FullName:      "test-org/forked-repo",
			CloneURL:      "https://gitea.example.com/test-org/forked-repo.git",
			SSHURL:        "git@gitea.example.com:test-org/forked-repo.git",
			Fork:          true,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
		{
			ID:            3,
			Name:          "another-original",
			FullName:      "test-org/another-original",
			CloneURL:      "https://gitea.example.com/test-org/another-original.git",
			SSHURL:        "git@gitea.example.com:test-org/another-original.git",
			Fork:          false,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
	}

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_SKIP_FORKS", "true")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_SKIP_FORKS")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 repositories (fork excluded), got %d", len(result))
	}

	for _, repo := range result {
		if repo.Name == "forked-repo" {
			t.Errorf("Forked repo should have been excluded, but found %q", repo.Name)
		}
	}
}

func TestGitea_GetOrgRepos_SSHProtocol(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	repos := []*gitea.Repository{
		mockGiteaRepository(1, "ssh-repo-1"),
		mockGiteaRepository(2, "ssh-repo-2"),
	}

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 repositories, got %d", len(result))
	}

	for _, repo := range result {
		expectedSSHPrefix := "git@gitea.example.com:"
		if !strings.HasPrefix(repo.CloneURL, expectedSSHPrefix) {
			t.Errorf("Expected CloneURL to start with %q, got %q", expectedSSHPrefix, repo.CloneURL)
		}
		if !strings.HasPrefix(repo.URL, expectedSSHPrefix) {
			t.Errorf("Expected URL to start with %q, got %q", expectedSSHPrefix, repo.URL)
		}
	}
}

func TestGitea_GetOrgRepos_BranchOverride(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	repos := []*gitea.Repository{
		mockGiteaRepository(1, "branch-repo-1"),
		mockGiteaRepository(2, "branch-repo-2"),
	}

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_BRANCH", "custom-branch")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_BRANCH")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 repositories, got %d", len(result))
	}

	for _, repo := range result {
		if repo.CloneBranch != "custom-branch" {
			t.Errorf("Expected CloneBranch %q for repo %q, got %q", "custom-branch", repo.Name, repo.CloneBranch)
		}
	}
}

func TestGitea_GetOrgRepos_WikiClone(t *testing.T) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	repos := []*gitea.Repository{
		{
			ID:            1,
			Name:          "wiki-repo",
			FullName:      "test-org/wiki-repo",
			CloneURL:      "https://gitea.example.com/test-org/wiki-repo.git",
			SSHURL:        "git@gitea.example.com:test-org/wiki-repo.git",
			HasWiki:       true,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
		{
			ID:            2,
			Name:          "no-wiki-repo",
			FullName:      "test-org/no-wiki-repo",
			CloneURL:      "https://gitea.example.com/test-org/no-wiki-repo.git",
			SSHURL:        "git@gitea.example.com:test-org/no-wiki-repo.git",
			HasWiki:       false,
			DefaultBranch: "main",
			Owner:         &gitea.User{UserName: "test-org"},
		},
	}

	mux.HandleFunc("/api/v1/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_CLONE_WIKI", "true")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_CLONE_WIKI")

	result, err := client.GetOrgRepos("test-org")
	if err != nil {
		t.Fatalf("GetOrgRepos failed: %v", err)
	}

	// Expect 3 results: wiki-repo, wiki-repo.wiki, no-wiki-repo
	if len(result) != 3 {
		t.Fatalf("Expected 3 entries (2 repos + 1 wiki), got %d", len(result))
	}

	foundWiki := false
	for _, repo := range result {
		if repo.IsWiki {
			foundWiki = true
			if repo.Path != "wiki-repo.wiki" {
				t.Errorf("Expected wiki Path %q, got %q", "wiki-repo.wiki", repo.Path)
			}
			expectedWikiURL := "https://gitea.example.com/test-org/wiki-repo.wiki.git"
			if repo.CloneURL != expectedWikiURL {
				t.Errorf("Expected wiki CloneURL %q, got %q", expectedWikiURL, repo.CloneURL)
			}
			if repo.CloneBranch != "master" {
				t.Errorf("Expected wiki CloneBranch %q, got %q", "master", repo.CloneBranch)
			}
		}
	}

	if !foundWiki {
		t.Error("Expected a wiki entry in results, but none was found")
	}
}

// Benchmark test to ensure pagination doesn't cause performance issues
func BenchmarkGitea_GetOrgRepos_LargePagination(b *testing.B) {
	client, mux, _, teardown := setupGiteaTest()
	defer teardown()

	totalRepos := 500 // Large number of repositories

	mux.HandleFunc("/api/v1/orgs/large-org/repos", func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if pageParam := r.URL.Query().Get("page"); pageParam != "" {
			fmt.Sscanf(pageParam, "%d", &page)
		}

		startIdx := (page - 1) * client.perPage
		endIdx := startIdx + client.perPage
		if endIdx > totalRepos {
			endIdx = totalRepos
		}

		repos := make([]*gitea.Repository, 0, endIdx-startIdx)
		for i := startIdx; i < endIdx; i++ {
			repos = append(repos, mockGiteaRepository(int64(i+1), fmt.Sprintf("bench-repo-%03d", i+1)))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := client.GetOrgRepos("large-org")
		if err != nil {
			b.Fatalf("GetOrgRepos failed: %v", err)
		}
		if len(result) != totalRepos {
			b.Errorf("Expected %d repositories, got %d", totalRepos, len(result))
		}
	}
}
