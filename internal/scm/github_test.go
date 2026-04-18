package scm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	ghpkg "github.com/google/go-github/v84/github"
)

const (
	// baseURLPath is a non-empty Client.BaseURL path to use during tests,
	// to ensure relative URLs are used for all endpoints.
	baseURLPath = "/api-v3"
)

func setup() (client *ghpkg.Client, mux *http.ServeMux, serverURL string, teardown func()) {
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()

	// We want to ensure that tests catch mistakes where the endpoint URL is
	// specified as absolute rather than relative. It only makes a difference
	// when there's a non-empty base URL path. So, use that.
	apiHandler := http.NewServeMux()
	apiHandler.Handle(baseURLPath+"/", http.StripPrefix(baseURLPath, mux))
	apiHandler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(os.Stderr, "FAIL: Client.BaseURL path prefix is not preserved in the request URL:")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\t"+req.URL.String())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "\tDid you accidentally use an absolute endpoint URL rather than relative?")
		fmt.Fprintln(os.Stderr, "\tSee https://github.com/google/go-github/issues/752 for information.")
		http.Error(w, "Client.BaseURL path prefix is not preserved in the request URL.", http.StatusInternalServerError)
	})

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	// client is the GitHub client being tested and is
	// configured to use test server.
	client = ghpkg.NewClient(nil)
	url, _ := url.Parse(server.URL + baseURLPath + "/")
	client.BaseURL = url
	client.UploadURL = url

	return client, mux, server.URL, server.Close
}

func TestGetOrgRepos(t *testing.T) {
	client, mux, _, teardown := setup()

	github := Github{Client: client}

	defer teardown()

	mux.HandleFunc("/orgs/testorg/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"id":1, "clone_url": "https://example.com", "name": "foobar1", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":2, "clone_url": "https://example.com", "name": "foobar2", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":3, "clone_url": "https://example.com", "name": "foobar3", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":4, "clone_url": "https://example.com", "name": "foobar4", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":5, "clone_url": "https://example.com", "name": "foobar5", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":6, "clone_url": "https://example.com", "name": "foobar6", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":7, "clone_url": "https://example.com", "name": "foobar7", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":8, "clone_url": "https://example.com", "name": "tp-foobar8", "archived": false, "fork": false, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":9, "clone_url": "https://example.com", "name": "tp-foobar9", "archived": false, "fork": true, "topics": ["a","b","c"], "ssh_url": "git://example.com"},
			{"id":10, "clone_url": "https://example.com", "name": "tp-foobar10", "archived": true, "fork": false, "topics": ["test-topic"], "ssh_url": "httgitps://example.com"}
			]`)
	})

	t.Run("Should return all repos", func(tt *testing.T) {

		resp, err := github.GetOrgRepos("testorg")

		if err != nil {
			t.Fatal(err)
		}

		want := 10
		got := len(resp)
		if want != got {
			tt.Errorf("Expected %v repo, got: %v", want, got)
		}

	})

	t.Run("Should skip archived repos when env is set", func(tt *testing.T) {
		os.Setenv("GHORG_SKIP_ARCHIVED", "true")
		resp, err := github.GetOrgRepos("testorg")

		if err != nil {
			t.Fatal(err)
		}
		want := 9
		got := len(resp)
		if want != got {
			tt.Errorf("Expected %v repo, got: %v", want, got)
		}
		os.Setenv("GHORG_SKIP_ARCHIVED", "")

	})

	t.Run("Should skip forked repos when env is set", func(tt *testing.T) {
		os.Setenv("GHORG_SKIP_FORKS", "true")
		resp, err := github.GetOrgRepos("testorg")

		if err != nil {
			t.Fatal(err)
		}
		want := 9
		got := len(resp)
		if want != got {
			tt.Errorf("Expected %v repo, got: %v", want, got)
		}
		os.Setenv("GHORG_SKIP_FORKS", "")

	})

	t.Run("Find all repos with specific topic set", func(tt *testing.T) {
		os.Setenv("GHORG_TOPICS", "test-topic")
		resp, err := github.GetOrgRepos("testorg")

		if err != nil {
			t.Fatal(err)
		}
		want := 1
		got := len(resp)
		if want != got {
			tt.Errorf("Expected %v repo, got: %v", want, got)
		}
		os.Setenv("GHORG_TOPICS", "")
	})
}

func TestGetUserRepos(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_GITHUB_TOKEN")

	// Mock the /user endpoint for SetTokensUsername
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login": "tokenowner"}`)
	})

	// Mock the /users/testuser/repos endpoint
	mux.HandleFunc("/users/testuser/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"id":1, "clone_url": "https://github.com/testuser/repo1.git", "name": "repo1", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:testuser/repo1.git", "owner": {"login": "testuser", "type": "User"}, "default_branch": "main", "has_wiki": false},
			{"id":2, "clone_url": "https://github.com/testuser/repo2.git", "name": "repo2", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:testuser/repo2.git", "owner": {"login": "testuser", "type": "User"}, "default_branch": "main", "has_wiki": false},
			{"id":3, "clone_url": "https://github.com/someorg/repo3.git", "name": "repo3", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:someorg/repo3.git", "owner": {"login": "someorg", "type": "Organization"}, "default_branch": "main", "has_wiki": false}
		]`)
	})

	github := Github{Client: client, perPage: 10}

	t.Run("Should return only user-owned repos for non-token user", func(tt *testing.T) {
		resp, err := github.GetUserRepos("testuser")
		if err != nil {
			tt.Fatalf("unexpected error: %v", err)
		}

		// repo3 has owner type "Organization" so it should be filtered out
		want := 2
		got := len(resp)
		if want != got {
			tt.Errorf("Expected %v repos, got: %v", want, got)
		}
	})

	t.Run("Repos should have HTTPS clone URLs with token", func(tt *testing.T) {
		resp, err := github.GetUserRepos("testuser")
		if err != nil {
			tt.Fatalf("unexpected error: %v", err)
		}

		if len(resp) == 0 {
			tt.Fatal("expected at least one repo")
		}

		for _, repo := range resp {
			if !strings.Contains(repo.CloneURL, "test-token@") {
				tt.Errorf("Expected clone URL to contain token, got: %s", repo.CloneURL)
			}
		}
	})
}

func TestGetUserRepos_AuthenticatedUser(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_GITHUB_TOKEN")

	// Mock the /user endpoint - returns "authuser" as login
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login": "authuser"}`)
	})

	// Mock the /user/repos endpoint for authenticated user
	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"id":1, "clone_url": "https://github.com/authuser/myrepo.git", "name": "myrepo", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:authuser/myrepo.git", "owner": {"login": "authuser", "type": "User"}, "default_branch": "main", "has_wiki": false},
			{"id":2, "clone_url": "https://github.com/someorg/orgcontrib.git", "name": "orgcontrib", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:someorg/orgcontrib.git", "owner": {"login": "someorg", "type": "Organization"}, "default_branch": "develop", "has_wiki": false}
		]`)
	})

	github := Github{Client: client, perPage: 10}

	t.Run("Should return all repos for authenticated user without owner filtering", func(tt *testing.T) {
		// Reset tokenUsername so SetTokensUsername will set it from /user
		tokenUsername = ""

		resp, err := github.GetUserRepos("authuser")
		if err != nil {
			tt.Fatalf("unexpected error: %v", err)
		}

		// When targetUser == tokenUsername, no owner filtering occurs
		want := 2
		got := len(resp)
		if want != got {
			tt.Errorf("Expected %v repos, got: %v", want, got)
		}
	})
}

func TestGetOrgRepos_MultiPage(t *testing.T) {
	client, mux, serverURL, teardown := setup()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_GITHUB_TOKEN")

	// Mock the /user endpoint for SetTokensUsername
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"login": "testuser"}`)
	})

	mux.HandleFunc("/orgs/bigorg/repos", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")

		switch page {
		case "", "1":
			// First page - set Link header indicating 3 total pages
			linkURL := serverURL + baseURLPath + "/orgs/bigorg/repos?page=3"
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="last"`, linkURL))
			fmt.Fprint(w, `[
				{"id":1, "clone_url": "https://github.com/bigorg/repo1.git", "name": "repo1", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:bigorg/repo1.git", "default_branch": "main", "has_wiki": false},
				{"id":2, "clone_url": "https://github.com/bigorg/repo2.git", "name": "repo2", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:bigorg/repo2.git", "default_branch": "main", "has_wiki": false}
			]`)
		case "2":
			fmt.Fprint(w, `[
				{"id":3, "clone_url": "https://github.com/bigorg/repo3.git", "name": "repo3", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:bigorg/repo3.git", "default_branch": "main", "has_wiki": false},
				{"id":4, "clone_url": "https://github.com/bigorg/repo4.git", "name": "repo4", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:bigorg/repo4.git", "default_branch": "main", "has_wiki": false}
			]`)
		case "3":
			fmt.Fprint(w, `[
				{"id":5, "clone_url": "https://github.com/bigorg/repo5.git", "name": "repo5", "archived": false, "fork": false, "topics": [], "ssh_url": "git@github.com:bigorg/repo5.git", "default_branch": "main", "has_wiki": false}
			]`)
		}
	})

	github := Github{Client: client, perPage: 2}

	t.Run("Should fetch all repos across multiple pages", func(tt *testing.T) {
		resp, err := github.GetOrgRepos("bigorg")
		if err != nil {
			tt.Fatalf("unexpected error: %v", err)
		}

		want := 5
		got := len(resp)
		if want != got {
			tt.Errorf("Expected %v repos, got: %v", want, got)
		}
	})

	t.Run("Should contain repos from all pages", func(tt *testing.T) {
		resp, err := github.GetOrgRepos("bigorg")
		if err != nil {
			tt.Fatalf("unexpected error: %v", err)
		}

		names := make(map[string]bool)
		for _, repo := range resp {
			names[repo.Name] = true
		}

		for _, expected := range []string{"repo1", "repo2", "repo3", "repo4", "repo5"} {
			if !names[expected] {
				tt.Errorf("Expected repo %q to be present in results", expected)
			}
		}
	})
}

func TestAddTokenToHTTPSCloneURL(t *testing.T) {
	// Save and restore the package-level tokenUsername
	origTokenUsername := tokenUsername
	defer func() { tokenUsername = origTokenUsername }()

	tokenUsername = "testuser"
	gh := Github{}

	tests := []struct {
		name     string
		url      string
		token    string
		expected string
	}{
		{
			name:     "Standard GitHub HTTPS URL",
			url:      "https://github.com/org/repo.git",
			token:    "ghp_abc123",
			expected: "https://testuser:ghp_abc123@github.com/org/repo.git",
		},
		{
			name:     "GitHub Enterprise HTTPS URL with path",
			url:      "https://github.example.com/org/suborg/repo.git",
			token:    "token-xyz",
			expected: "https://testuser:token-xyz@github.example.com/org/suborg/repo.git",
		},
		{
			name:     "URL with nested path",
			url:      "https://github.com/deep/nested/path/repo.git",
			token:    "tok",
			expected: "https://testuser:tok@github.com/deep/nested/path/repo.git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(tt *testing.T) {
			got := gh.addTokenToHTTPSCloneURL(tc.url, tc.token)
			if got != tc.expected {
				tt.Errorf("Expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestGithub_Filter(t *testing.T) {
	// Save and restore package-level tokenUsername
	origTokenUsername := tokenUsername
	defer func() { tokenUsername = origTokenUsername }()
	tokenUsername = "testuser"

	gh := Github{}

	// Helper to create test repos
	boolPtr := func(b bool) *bool { return &b }
	strPtr := func(s string) *string { return &s }

	makeRepo := func(name, cloneURL, sshURL, lang, defaultBranch string, archived, fork, hasWiki bool, topics []string) *ghpkg.Repository {
		r := &ghpkg.Repository{
			Name:          strPtr(name),
			CloneURL:      strPtr(cloneURL),
			SSHURL:        strPtr(sshURL),
			Archived:      boolPtr(archived),
			Fork:          boolPtr(fork),
			HasWiki:       boolPtr(hasWiki),
			Topics:        topics,
			DefaultBranch: strPtr(defaultBranch),
		}
		if lang != "" {
			r.Language = strPtr(lang)
		}
		return r
	}

	t.Run("Language filter includes matching repos", func(tt *testing.T) {
		os.Setenv("GHORG_GITHUB_FILTER_LANGUAGE", "go")
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")
		defer os.Unsetenv("GHORG_GITHUB_FILTER_LANGUAGE")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
		defer os.Unsetenv("GHORG_GITHUB_TOKEN")

		repos := []*ghpkg.Repository{
			makeRepo("go-repo", "https://github.com/org/go-repo.git", "git@github.com:org/go-repo.git", "Go", "main", false, false, false, nil),
			makeRepo("py-repo", "https://github.com/org/py-repo.git", "git@github.com:org/py-repo.git", "Python", "main", false, false, false, nil),
			makeRepo("another-go", "https://github.com/org/another-go.git", "git@github.com:org/another-go.git", "Go", "main", false, false, false, nil),
		}

		result := gh.filter(repos)

		want := 2
		got := len(result)
		if want != got {
			tt.Errorf("Expected %v repos, got: %v", want, got)
		}
	})

	t.Run("Language filter with multiple languages", func(tt *testing.T) {
		os.Setenv("GHORG_GITHUB_FILTER_LANGUAGE", "go,python")
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")
		defer os.Unsetenv("GHORG_GITHUB_FILTER_LANGUAGE")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
		defer os.Unsetenv("GHORG_GITHUB_TOKEN")

		repos := []*ghpkg.Repository{
			makeRepo("go-repo", "https://github.com/org/go-repo.git", "git@github.com:org/go-repo.git", "Go", "main", false, false, false, nil),
			makeRepo("py-repo", "https://github.com/org/py-repo.git", "git@github.com:org/py-repo.git", "Python", "main", false, false, false, nil),
			makeRepo("rust-repo", "https://github.com/org/rust-repo.git", "git@github.com:org/rust-repo.git", "Rust", "main", false, false, false, nil),
		}

		result := gh.filter(repos)

		want := 2
		got := len(result)
		if want != got {
			tt.Errorf("Expected %v repos, got: %v", want, got)
		}
	})

	t.Run("Language filter excludes repos with no language set", func(tt *testing.T) {
		os.Setenv("GHORG_GITHUB_FILTER_LANGUAGE", "go")
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")
		defer os.Unsetenv("GHORG_GITHUB_FILTER_LANGUAGE")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
		defer os.Unsetenv("GHORG_GITHUB_TOKEN")

		repos := []*ghpkg.Repository{
			makeRepo("go-repo", "https://github.com/org/go-repo.git", "git@github.com:org/go-repo.git", "Go", "main", false, false, false, nil),
			makeRepo("nolang-repo", "https://github.com/org/nolang-repo.git", "git@github.com:org/nolang-repo.git", "", "main", false, false, false, nil),
		}

		result := gh.filter(repos)

		want := 1
		got := len(result)
		if want != got {
			tt.Errorf("Expected %v repos, got: %v", want, got)
		}
	})

	t.Run("Wiki cloning adds wiki repo entries", func(tt *testing.T) {
		os.Setenv("GHORG_CLONE_WIKI", "true")
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		os.Setenv("GHORG_GITHUB_TOKEN", "test-token")
		defer os.Unsetenv("GHORG_CLONE_WIKI")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
		defer os.Unsetenv("GHORG_GITHUB_TOKEN")

		repos := []*ghpkg.Repository{
			makeRepo("wiki-repo", "https://github.com/org/wiki-repo.git", "git@github.com:org/wiki-repo.git", "", "main", false, false, true, nil),
			makeRepo("no-wiki-repo", "https://github.com/org/no-wiki-repo.git", "git@github.com:org/no-wiki-repo.git", "", "main", false, false, false, nil),
		}

		result := gh.filter(repos)

		// wiki-repo + wiki-repo.wiki + no-wiki-repo = 3
		want := 3
		got := len(result)
		if want != got {
			tt.Errorf("Expected %v entries, got: %v", want, got)
		}

		// Verify wiki entry properties
		foundWiki := false
		for _, repo := range result {
			if repo.IsWiki {
				foundWiki = true
				if !strings.Contains(repo.CloneURL, ".wiki.git") {
					tt.Errorf("Wiki clone URL should contain .wiki.git, got: %s", repo.CloneURL)
				}
				if repo.CloneBranch != "master" {
					tt.Errorf("Wiki clone branch should be 'master', got: %s", repo.CloneBranch)
				}
				if repo.Path != "wiki-repo.wiki" {
					tt.Errorf("Wiki path should be 'wiki-repo.wiki', got: %s", repo.Path)
				}
			}
		}
		if !foundWiki {
			tt.Error("Expected to find a wiki entry in results")
		}
	})

	t.Run("SSH clone protocol uses SSH URL", func(tt *testing.T) {
		os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

		repos := []*ghpkg.Repository{
			makeRepo("ssh-repo", "https://github.com/org/ssh-repo.git", "git@github.com:org/ssh-repo.git", "", "main", false, false, false, nil),
		}

		result := gh.filter(repos)

		if len(result) != 1 {
			tt.Fatalf("Expected 1 repo, got: %v", len(result))
		}

		if result[0].CloneURL != "git@github.com:org/ssh-repo.git" {
			tt.Errorf("Expected SSH clone URL, got: %s", result[0].CloneURL)
		}
	})

	t.Run("Custom branch overrides default branch", func(tt *testing.T) {
		os.Setenv("GHORG_BRANCH", "develop")
		os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
		defer os.Unsetenv("GHORG_BRANCH")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

		repos := []*ghpkg.Repository{
			makeRepo("branch-repo", "https://github.com/org/branch-repo.git", "git@github.com:org/branch-repo.git", "", "main", false, false, false, nil),
		}

		result := gh.filter(repos)

		if len(result) != 1 {
			tt.Fatalf("Expected 1 repo, got: %v", len(result))
		}

		if result[0].CloneBranch != "develop" {
			tt.Errorf("Expected branch 'develop', got: %s", result[0].CloneBranch)
		}
	})

	t.Run("Default branch used when GHORG_BRANCH not set", func(tt *testing.T) {
		os.Unsetenv("GHORG_BRANCH")
		os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

		repos := []*ghpkg.Repository{
			makeRepo("default-branch-repo", "https://github.com/org/default-branch-repo.git", "git@github.com:org/default-branch-repo.git", "", "develop", false, false, false, nil),
		}

		result := gh.filter(repos)

		if len(result) != 1 {
			tt.Fatalf("Expected 1 repo, got: %v", len(result))
		}

		if result[0].CloneBranch != "develop" {
			tt.Errorf("Expected branch 'develop', got: %s", result[0].CloneBranch)
		}
	})

	t.Run("Empty default branch falls back to master", func(tt *testing.T) {
		os.Unsetenv("GHORG_BRANCH")
		os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

		repos := []*ghpkg.Repository{
			makeRepo("no-default-repo", "https://github.com/org/no-default-repo.git", "git@github.com:org/no-default-repo.git", "", "", false, false, false, nil),
		}

		result := gh.filter(repos)

		if len(result) != 1 {
			tt.Fatalf("Expected 1 repo, got: %v", len(result))
		}

		if result[0].CloneBranch != "master" {
			tt.Errorf("Expected branch 'master' as fallback, got: %s", result[0].CloneBranch)
		}
	})
}

func TestSetTokensUsername(t *testing.T) {
	// Save and restore the package-level tokenUsername
	origTokenUsername := tokenUsername
	defer func() { tokenUsername = origTokenUsername }()

	t.Run("GitHub App token sets x-access-token", func(tt *testing.T) {
		tokenUsername = ""
		os.Setenv("GHORG_GITHUB_TOKEN_FROM_GITHUB_APP", "true")
		defer os.Unsetenv("GHORG_GITHUB_TOKEN_FROM_GITHUB_APP")

		client, _, _, teardown := setup()
		defer teardown()

		gh := Github{Client: client}
		gh.SetTokensUsername()

		if tokenUsername != "x-access-token" {
			tt.Errorf("Expected tokenUsername to be 'x-access-token', got: %s", tokenUsername)
		}
	})

	t.Run("PEM path set uses x-access-token", func(tt *testing.T) {
		tokenUsername = ""
		os.Setenv("GHORG_GITHUB_APP_PEM_PATH", "/some/path/to/key.pem")
		defer os.Unsetenv("GHORG_GITHUB_APP_PEM_PATH")

		client, _, _, teardown := setup()
		defer teardown()

		gh := Github{Client: client}
		gh.SetTokensUsername()

		if tokenUsername != "x-access-token" {
			tt.Errorf("Expected tokenUsername to be 'x-access-token', got: %s", tokenUsername)
		}
	})

	t.Run("Normal token fetches username from API", func(tt *testing.T) {
		tokenUsername = ""
		os.Unsetenv("GHORG_GITHUB_TOKEN_FROM_GITHUB_APP")
		os.Unsetenv("GHORG_GITHUB_APP_PEM_PATH")

		client, mux, _, teardown := setup()
		defer teardown()

		mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"login": "apiuser"}`)
		})

		gh := Github{Client: client}
		gh.SetTokensUsername()

		if tokenUsername != "apiuser" {
			tt.Errorf("Expected tokenUsername to be 'apiuser', got: %s", tokenUsername)
		}
	})

	t.Run("Failed API call falls back to x-access-token", func(tt *testing.T) {
		tokenUsername = ""
		os.Unsetenv("GHORG_GITHUB_TOKEN_FROM_GITHUB_APP")
		os.Unsetenv("GHORG_GITHUB_APP_PEM_PATH")

		client, mux, _, teardown := setup()
		defer teardown()

		mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})

		gh := Github{Client: client}
		gh.SetTokensUsername()

		if tokenUsername != "x-access-token" {
			tt.Errorf("Expected tokenUsername to be 'x-access-token' on API failure, got: %s", tokenUsername)
		}
	})

	t.Run("Empty login falls back to x-access-token", func(tt *testing.T) {
		tokenUsername = ""
		os.Unsetenv("GHORG_GITHUB_TOKEN_FROM_GITHUB_APP")
		os.Unsetenv("GHORG_GITHUB_APP_PEM_PATH")

		client, mux, _, teardown := setup()
		defer teardown()

		mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"login": ""}`)
		})

		gh := Github{Client: client}
		gh.SetTokensUsername()

		if tokenUsername != "x-access-token" {
			tt.Errorf("Expected tokenUsername to be 'x-access-token' when login is empty, got: %s", tokenUsername)
		}
	})
}
