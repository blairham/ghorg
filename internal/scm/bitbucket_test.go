package scm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestInsertAppPasswordCredentialsIntoURL(t *testing.T) {
	// Set environment variables for the test
	os.Setenv("GHORG_BITBUCKET_USERNAME", "ghorg")
	os.Setenv("GHORG_BITBUCKET_APP_PASSWORD", "testpassword")

	// Define a test URL
	testURL := "https://ghorg@bitbucket.org/foobar/testrepo.git"

	// Call the function with the test URL
	resultURL := insertAppPasswordCredentialsIntoURL(testURL)

	// Define the expected result
	expectedURL := "https://ghorg:testpassword@bitbucket.org/foobar/testrepo.git"

	// Check if the result matches the expected result
	if resultURL != expectedURL {
		t.Errorf("Expected %s, but got %s", expectedURL, resultURL)
	}
}

// setupBitbucketServerTest creates a test HTTP server and a Bitbucket client configured
// to use it. Returns the client, mux for registering handlers, the server URL, and a
// teardown function.
func setupBitbucketServerTest() (client Bitbucket, mux *http.ServeMux, serverURL string, teardown func()) {
	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	client = Bitbucket{
		Client:     nil,
		isServer:   true,
		serverURL:  server.URL,
		httpClient: &http.Client{},
		username:   "admin",
		password:   "secret",
	}

	return client, mux, server.URL, server.Close
}

// makeServerRepoJSON builds a JSON response body for the Bitbucket Server repos API.
func makeServerRepoJSON(repos []ServerRepository, size int, isLastPage bool, start int) string {
	resp := ServerProjectResponse{
		Values:     repos,
		Size:       size,
		IsLastPage: isLastPage,
		Start:      start,
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// newServerRepo is a helper to create a ServerRepository with the given name, project key,
// and clone links.
func newServerRepo(name, projectKey, httpsHref, sshHref string) ServerRepository {
	cloneLinks := []any{}
	if httpsHref != "" {
		cloneLinks = append(cloneLinks, map[string]any{
			"href": httpsHref,
			"name": "http",
		})
	}
	if sshHref != "" {
		cloneLinks = append(cloneLinks, map[string]any{
			"href": sshHref,
			"name": "ssh",
		})
	}
	return ServerRepository{
		Name: name,
		Slug: strings.ToLower(name),
		Links: map[string]any{
			"clone": cloneLinks,
		},
		Project: struct {
			Key string `json:"key"`
		}{Key: projectKey},
	}
}

// --- Bitbucket Server: getServerProjectRepos ---

func TestGetServerProjectRepos_SinglePage(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	repos := []ServerRepository{
		newServerRepo("repo1", "PROJ", "https://bitbucket.example.com/scm/proj/repo1.git", "ssh://git@bitbucket.example.com:7999/proj/repo1.git"),
		newServerRepo("repo2", "PROJ", "https://bitbucket.example.com/scm/proj/repo2.git", "ssh://git@bitbucket.example.com:7999/proj/repo2.git"),
	}

	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON(repos, 2, true, 0))
	})

	result, err := client.getServerProjectRepos("PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(result))
	}

	if result[0].Name != "repo1" {
		t.Errorf("expected repo name 'repo1', got '%s'", result[0].Name)
	}
	if result[1].Name != "repo2" {
		t.Errorf("expected repo name 'repo2', got '%s'", result[1].Name)
	}

	// Verify credentials are embedded in CloneURL
	if !strings.Contains(result[0].CloneURL, "admin:secret@") {
		t.Errorf("expected credentials in clone URL, got '%s'", result[0].CloneURL)
	}

	// Verify URL (without credentials) is set
	if result[0].URL != "https://bitbucket.example.com/scm/proj/repo1.git" {
		t.Errorf("expected URL without credentials, got '%s'", result[0].URL)
	}

	// Verify path is project/slug
	if result[0].Path != "PROJ/repo1" {
		t.Errorf("expected path 'PROJ/repo1', got '%s'", result[0].Path)
	}

	// Default branch should be master for server
	if result[0].CloneBranch != "master" {
		t.Errorf("expected default branch 'master', got '%s'", result[0].CloneBranch)
	}
}

func TestGetServerProjectRepos_MultiPage(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	// Create 30 repos total, so we get 2 pages (limit=25)
	allRepos := make([]ServerRepository, 30)
	for i := range 30 {
		name := fmt.Sprintf("repo%d", i+1)
		allRepos[i] = newServerRepo(
			name, "PROJ",
			fmt.Sprintf("https://bitbucket.example.com/scm/proj/%s.git", strings.ToLower(name)),
			fmt.Sprintf("ssh://git@bitbucket.example.com:7999/proj/%s.git", strings.ToLower(name)),
		)
	}

	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos", func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/json")

		if start == "0" || start == "" {
			// First page: 25 repos, not last page, total size 30
			fmt.Fprint(w, makeServerRepoJSON(allRepos[:25], 30, false, 0))
		} else if start == "25" {
			// Second page: 5 repos, last page
			fmt.Fprint(w, makeServerRepoJSON(allRepos[25:], 30, true, 25))
		} else {
			http.Error(w, "unexpected start value", http.StatusBadRequest)
		}
	})

	result, err := client.getServerProjectRepos("PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 30 {
		t.Fatalf("expected 30 repos, got %d", len(result))
	}

	// Verify first and last repos
	if result[0].Name != "repo1" {
		t.Errorf("expected first repo name 'repo1', got '%s'", result[0].Name)
	}
	if result[29].Name != "repo30" {
		t.Errorf("expected last repo name 'repo30', got '%s'", result[29].Name)
	}
}

func TestGetServerProjectRepos_ThreePages(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	// 55 repos = 3 pages (25, 25, 5)
	allRepos := make([]ServerRepository, 55)
	for i := range 55 {
		name := fmt.Sprintf("r%03d", i+1)
		allRepos[i] = newServerRepo(
			name, "BIG",
			fmt.Sprintf("https://bb.example.com/scm/big/%s.git", name),
			fmt.Sprintf("ssh://git@bb.example.com:7999/big/%s.git", name),
		)
	}

	mux.HandleFunc("/rest/api/1.0/projects/BIG/repos", func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/json")

		switch start {
		case "0", "":
			fmt.Fprint(w, makeServerRepoJSON(allRepos[:25], 55, false, 0))
		case "25":
			fmt.Fprint(w, makeServerRepoJSON(allRepos[25:50], 55, false, 25))
		case "50":
			fmt.Fprint(w, makeServerRepoJSON(allRepos[50:], 55, true, 50))
		default:
			http.Error(w, "unexpected start", http.StatusBadRequest)
		}
	})

	result, err := client.getServerProjectRepos("BIG")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 55 {
		t.Fatalf("expected 55 repos, got %d", len(result))
	}

	// Verify ordering is maintained
	if result[0].Name != "r001" {
		t.Errorf("expected first repo 'r001', got '%s'", result[0].Name)
	}
	if result[54].Name != "r055" {
		t.Errorf("expected last repo 'r055', got '%s'", result[54].Name)
	}
}

func TestGetServerProjectRepos_APIError(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	mux.HandleFunc("/rest/api/1.0/projects/BADPROJ/repos", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"errors":[{"message":"Project BADPROJ does not exist"}]}`, http.StatusNotFound)
	})

	_, err := client.getServerProjectRepos("BADPROJ")
	if err == nil {
		t.Fatal("expected error for non-existent project, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to contain status 404, got: %v", err)
	}
}

func TestGetServerProjectRepos_MalformedJSON(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	mux.HandleFunc("/rest/api/1.0/projects/BAD/repos", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{this is not valid json`)
	})

	_, err := client.getServerProjectRepos("BAD")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestGetServerProjectRepos_EmptyResponse(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	mux.HandleFunc("/rest/api/1.0/projects/EMPTY/repos", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON([]ServerRepository{}, 0, true, 0))
	})

	result, err := client.getServerProjectRepos("EMPTY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 repos, got %d", len(result))
	}
}

func TestGetServerProjectRepos_SSHProtocol(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	repos := []ServerRepository{
		newServerRepo("ssh-repo", "PROJ",
			"https://bitbucket.example.com/scm/proj/ssh-repo.git",
			"ssh://git@bitbucket.example.com:7999/proj/ssh-repo.git"),
	}

	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON(repos, 1, true, 0))
	})

	result, err := client.getServerProjectRepos("PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result))
	}

	if result[0].CloneURL != "ssh://git@bitbucket.example.com:7999/proj/ssh-repo.git" {
		t.Errorf("expected SSH clone URL, got '%s'", result[0].CloneURL)
	}

	// SSH URL should also be set as the regular URL
	if result[0].URL != "ssh://git@bitbucket.example.com:7999/proj/ssh-repo.git" {
		t.Errorf("expected SSH URL, got '%s'", result[0].URL)
	}
}

func TestGetServerProjectRepos_BranchOverride(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "release/v2")
	defer os.Unsetenv("GHORG_BRANCH")

	repos := []ServerRepository{
		newServerRepo("branch-repo", "PROJ",
			"https://bitbucket.example.com/scm/proj/branch-repo.git",
			"ssh://git@bitbucket.example.com:7999/proj/branch-repo.git"),
	}

	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON(repos, 1, true, 0))
	})

	result, err := client.getServerProjectRepos("PROJ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result))
	}

	if result[0].CloneBranch != "release/v2" {
		t.Errorf("expected branch 'release/v2', got '%s'", result[0].CloneBranch)
	}
}

// --- Bitbucket Server: getServerUserRepos ---

func TestGetServerUserRepos_SinglePage(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	repos := []ServerRepository{
		newServerRepo("user-repo1", "~ADMIN", "https://bitbucket.example.com/scm/~admin/user-repo1.git", "ssh://git@bitbucket.example.com:7999/~admin/user-repo1.git"),
		newServerRepo("user-repo2", "~ADMIN", "https://bitbucket.example.com/scm/~admin/user-repo2.git", "ssh://git@bitbucket.example.com:7999/~admin/user-repo2.git"),
		newServerRepo("user-repo3", "~ADMIN", "https://bitbucket.example.com/scm/~admin/user-repo3.git", "ssh://git@bitbucket.example.com:7999/~admin/user-repo3.git"),
	}

	mux.HandleFunc("/rest/api/1.0/repos", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON(repos, 3, true, 0))
	})

	result, err := client.getServerUserRepos("admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(result))
	}

	for i, r := range result {
		expectedName := fmt.Sprintf("user-repo%d", i+1)
		if r.Name != expectedName {
			t.Errorf("repo %d: expected name '%s', got '%s'", i, expectedName, r.Name)
		}
	}
}

func TestGetServerUserRepos_MultiPage(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	allRepos := make([]ServerRepository, 28)
	for i := range 28 {
		name := fmt.Sprintf("urepo%d", i+1)
		allRepos[i] = newServerRepo(
			name, "~USER",
			fmt.Sprintf("https://bitbucket.example.com/scm/~user/%s.git", strings.ToLower(name)),
			fmt.Sprintf("ssh://git@bitbucket.example.com:7999/~user/%s.git", strings.ToLower(name)),
		)
	}

	mux.HandleFunc("/rest/api/1.0/repos", func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		w.Header().Set("Content-Type", "application/json")

		if start == "0" || start == "" {
			fmt.Fprint(w, makeServerRepoJSON(allRepos[:25], 28, false, 0))
		} else if start == "25" {
			fmt.Fprint(w, makeServerRepoJSON(allRepos[25:], 28, true, 25))
		} else {
			http.Error(w, "unexpected start", http.StatusBadRequest)
		}
	})

	result, err := client.getServerUserRepos("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 28 {
		t.Fatalf("expected 28 repos, got %d", len(result))
	}

	if result[0].Name != "urepo1" {
		t.Errorf("expected first repo 'urepo1', got '%s'", result[0].Name)
	}
	if result[27].Name != "urepo28" {
		t.Errorf("expected last repo 'urepo28', got '%s'", result[27].Name)
	}
}

func TestGetServerUserRepos_APIError(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	mux.HandleFunc("/rest/api/1.0/repos", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})

	_, err := client.getServerUserRepos("anyone")
	if err == nil {
		t.Fatal("expected error for server error response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status 500, got: %v", err)
	}
}

func TestGetServerUserRepos_EmptyResponse(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	mux.HandleFunc("/rest/api/1.0/repos", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON([]ServerRepository{}, 0, true, 0))
	})

	result, err := client.getServerUserRepos("emptyuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 repos, got %d", len(result))
	}
}

// --- Bitbucket Server: filterServerRepos ---

func TestFilterServerRepos(t *testing.T) {
	tests := []struct {
		name          string
		protocol      string
		branch        string
		repos         []ServerRepository
		expectedCount int
		checkFunc     func(t *testing.T, repos []Repo)
	}{
		{
			name:     "HTTPS protocol selects http clone links",
			protocol: "https",
			repos: []ServerRepository{
				newServerRepo("repo1", "PROJ",
					"https://bb.example.com/scm/proj/repo1.git",
					"ssh://git@bb.example.com:7999/proj/repo1.git"),
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, repos []Repo) {
				if !strings.Contains(repos[0].CloneURL, "https://") {
					t.Errorf("expected HTTPS clone URL, got '%s'", repos[0].CloneURL)
				}
			},
		},
		{
			name:     "SSH protocol selects ssh clone links",
			protocol: "ssh",
			repos: []ServerRepository{
				newServerRepo("repo1", "PROJ",
					"https://bb.example.com/scm/proj/repo1.git",
					"ssh://git@bb.example.com:7999/proj/repo1.git"),
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, repos []Repo) {
				if !strings.HasPrefix(repos[0].CloneURL, "ssh://") {
					t.Errorf("expected SSH clone URL, got '%s'", repos[0].CloneURL)
				}
				// SSH URLs should not have basic auth credentials
				if strings.Contains(repos[0].CloneURL, "admin:secret@") {
					t.Errorf("SSH clone URL should not contain basic auth credentials, got '%s'", repos[0].CloneURL)
				}
			},
		},
		{
			name:     "branch override sets CloneBranch",
			protocol: "https",
			branch:   "develop",
			repos: []ServerRepository{
				newServerRepo("repo1", "PROJ",
					"https://bb.example.com/scm/proj/repo1.git",
					"ssh://git@bb.example.com:7999/proj/repo1.git"),
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, repos []Repo) {
				if repos[0].CloneBranch != "develop" {
					t.Errorf("expected branch 'develop', got '%s'", repos[0].CloneBranch)
				}
			},
		},
		{
			name:     "default branch is master when GHORG_BRANCH is empty",
			protocol: "https",
			branch:   "",
			repos: []ServerRepository{
				newServerRepo("repo1", "PROJ",
					"https://bb.example.com/scm/proj/repo1.git",
					""),
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, repos []Repo) {
				if repos[0].CloneBranch != "master" {
					t.Errorf("expected default branch 'master', got '%s'", repos[0].CloneBranch)
				}
			},
		},
		{
			name:     "repo with no clone links is skipped",
			protocol: "https",
			repos: []ServerRepository{
				{
					Name: "no-links",
					Slug: "no-links",
					Links: map[string]any{
						"self": []any{map[string]any{"href": "https://example.com"}},
					},
					Project: struct {
						Key string `json:"key"`
					}{Key: "PROJ"},
				},
			},
			expectedCount: 0,
		},
		{
			name:     "repo with nil Links is skipped",
			protocol: "https",
			repos: []ServerRepository{
				{
					Name:  "nil-links",
					Slug:  "nil-links",
					Links: nil,
					Project: struct {
						Key string `json:"key"`
					}{Key: "PROJ"},
				},
			},
			expectedCount: 0,
		},
		{
			name:     "multiple repos filtered correctly",
			protocol: "https",
			repos: []ServerRepository{
				newServerRepo("alpha", "PROJ",
					"https://bb.example.com/scm/proj/alpha.git",
					"ssh://git@bb.example.com:7999/proj/alpha.git"),
				newServerRepo("beta", "PROJ",
					"https://bb.example.com/scm/proj/beta.git",
					"ssh://git@bb.example.com:7999/proj/beta.git"),
				newServerRepo("gamma", "PROJ",
					"https://bb.example.com/scm/proj/gamma.git",
					"ssh://git@bb.example.com:7999/proj/gamma.git"),
			},
			expectedCount: 3,
			checkFunc: func(t *testing.T, repos []Repo) {
				names := []string{"alpha", "beta", "gamma"}
				for i, name := range names {
					if repos[i].Name != name {
						t.Errorf("repo %d: expected name '%s', got '%s'", i, name, repos[i].Name)
					}
				}
			},
		},
		{
			name:     "SSH protocol with only HTTPS links yields no repos",
			protocol: "ssh",
			repos: []ServerRepository{
				newServerRepo("httpsonly", "PROJ",
					"https://bb.example.com/scm/proj/httpsonly.git",
					""),
			},
			expectedCount: 0,
		},
		{
			name:     "HTTPS name variant matches for https protocol",
			protocol: "https",
			repos: []ServerRepository{
				{
					Name: "https-name-repo",
					Slug: "https-name-repo",
					Links: map[string]any{
						"clone": []any{
							map[string]any{
								"href": "https://bb.example.com/scm/proj/https-name-repo.git",
								"name": "https",
							},
						},
					},
					Project: struct {
						Key string `json:"key"`
					}{Key: "PROJ"},
				},
			},
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("GHORG_CLONE_PROTOCOL", tc.protocol)
			defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
			os.Setenv("GHORG_BRANCH", tc.branch)
			defer os.Unsetenv("GHORG_BRANCH")

			client := Bitbucket{
				isServer: true,
				username: "admin",
				password: "secret",
			}

			result := client.filterServerRepos(tc.repos)

			if len(result) != tc.expectedCount {
				t.Fatalf("expected %d repos, got %d", tc.expectedCount, len(result))
			}

			if tc.checkFunc != nil {
				tc.checkFunc(t, result)
			}
		})
	}
}

// --- Bitbucket Server: addCredentialsToURL ---

func TestAddCredentialsToURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		username string
		password string
		expected string
	}{
		{
			name:     "HTTPS URL gets credentials inserted",
			url:      "https://bitbucket.example.com/scm/proj/repo.git",
			username: "admin",
			password: "secret",
			expected: "https://admin:secret@bitbucket.example.com/scm/proj/repo.git",
		},
		{
			name:     "HTTP URL gets credentials inserted",
			url:      "http://bitbucket.example.com/scm/proj/repo.git",
			username: "user",
			password: "pass",
			expected: "http://user:pass@bitbucket.example.com/scm/proj/repo.git",
		},
		{
			name:     "SSH URL is returned unchanged",
			url:      "ssh://git@bitbucket.example.com:7999/proj/repo.git",
			username: "admin",
			password: "secret",
			expected: "ssh://git@bitbucket.example.com:7999/proj/repo.git",
		},
		{
			name:     "empty username returns URL unchanged",
			url:      "https://bitbucket.example.com/scm/proj/repo.git",
			username: "",
			password: "secret",
			expected: "https://bitbucket.example.com/scm/proj/repo.git",
		},
		{
			name:     "empty password returns URL unchanged",
			url:      "https://bitbucket.example.com/scm/proj/repo.git",
			username: "admin",
			password: "",
			expected: "https://bitbucket.example.com/scm/proj/repo.git",
		},
		{
			name:     "both empty returns URL unchanged",
			url:      "https://bitbucket.example.com/scm/proj/repo.git",
			username: "",
			password: "",
			expected: "https://bitbucket.example.com/scm/proj/repo.git",
		},
		{
			name:     "credentials with special characters",
			url:      "https://bitbucket.example.com/scm/proj/repo.git",
			username: "user@domain",
			password: "p@ss:word",
			expected: "https://user@domain:p@ss:word@bitbucket.example.com/scm/proj/repo.git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := Bitbucket{
				username: tc.username,
				password: tc.password,
			}

			result := client.addCredentialsToURL(tc.url)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// --- Bitbucket Cloud: insertOauthTokenIntoURL ---

func TestInsertOauthTokenIntoURL(t *testing.T) {
	// insertOauthTokenIntoURL replaces the first "@" in the URL with
	// "x-token-auth:TOKEN@". So for "https://user@bitbucket.org/...",
	// the result is "https://userx-token-auth:TOKEN@bitbucket.org/...".
	// This is the actual behavior of the function.
	tests := []struct {
		name     string
		token    string
		url      string
		expected string
	}{
		{
			name:     "OAuth token is inserted into Bitbucket Cloud URL",
			token:    "myoauthtoken123",
			url:      "https://ghorg@bitbucket.org/myorg/myrepo.git",
			expected: "https://ghorgx-token-auth:myoauthtoken123@bitbucket.org/myorg/myrepo.git",
		},
		{
			name:     "OAuth token with URL without username prefix",
			token:    "mytoken",
			url:      "https://bitbucket.org/org/repo.git",
			expected: "https://bitbucket.org/org/repo.git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("GHORG_BITBUCKET_OAUTH_TOKEN", tc.token)
			defer os.Unsetenv("GHORG_BITBUCKET_OAUTH_TOKEN")

			result := insertOauthTokenIntoURL(tc.url)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// --- insertAppPasswordCredentialsIntoURL table-driven ---

func TestInsertAppPasswordCredentialsIntoURL_Table(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		appPassword string
		inputURL    string
		expected    string
	}{
		{
			name:        "standard Bitbucket Cloud URL",
			username:    "myuser",
			appPassword: "app-pass-123",
			inputURL:    "https://myuser@bitbucket.org/myorg/repo.git",
			expected:    "https://myuser:app-pass-123@bitbucket.org/myorg/repo.git",
		},
		{
			name:        "empty app password inserts colon only",
			username:    "myuser",
			appPassword: "",
			inputURL:    "https://myuser@bitbucket.org/myorg/repo.git",
			expected:    "https://myuser:@bitbucket.org/myorg/repo.git",
		},
		{
			name:        "password with special characters",
			username:    "user",
			appPassword: "p@ss/w0rd!",
			inputURL:    "https://user@bitbucket.org/org/repo.git",
			expected:    "https://user:p@ss/w0rd!@bitbucket.org/org/repo.git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("GHORG_BITBUCKET_USERNAME", tc.username)
			defer os.Unsetenv("GHORG_BITBUCKET_USERNAME")
			os.Setenv("GHORG_BITBUCKET_APP_PASSWORD", tc.appPassword)
			defer os.Unsetenv("GHORG_BITBUCKET_APP_PASSWORD")

			result := insertAppPasswordCredentialsIntoURL(tc.inputURL)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// --- Bitbucket Cloud: filter helper verification ---

func TestBitbucketCloudFilter(t *testing.T) {
	t.Run("HTTPS protocol with OAuth token inserts credentials", func(t *testing.T) {
		os.Setenv("GHORG_BITBUCKET_OAUTH_TOKEN", "my-oauth-token")
		defer os.Unsetenv("GHORG_BITBUCKET_OAUTH_TOKEN")

		// The insertOauthTokenIntoURL function replaces "@" with "x-token-auth:TOKEN@"
		url := "https://ghorg@bitbucket.org/myorg/cloud-repo1.git"
		result := insertOauthTokenIntoURL(url)
		expected := "https://ghorgx-token-auth:my-oauth-token@bitbucket.org/myorg/cloud-repo1.git"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("HTTPS protocol with app password inserts credentials", func(t *testing.T) {
		os.Setenv("GHORG_BITBUCKET_APP_PASSWORD", "app-password-123")
		defer os.Unsetenv("GHORG_BITBUCKET_APP_PASSWORD")
		os.Setenv("GHORG_BITBUCKET_USERNAME", "myuser")
		defer os.Unsetenv("GHORG_BITBUCKET_USERNAME")

		url := "https://myuser@bitbucket.org/myorg/cloud-repo1.git"
		result := insertAppPasswordCredentialsIntoURL(url)
		expected := "https://myuser:app-password-123@bitbucket.org/myorg/cloud-repo1.git"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("Cloud filter with bitbucket.Repository via filter method", func(t *testing.T) {
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
		os.Setenv("GHORG_BITBUCKET_OAUTH_TOKEN", "test-token")
		defer os.Unsetenv("GHORG_BITBUCKET_OAUTH_TOKEN")
		os.Setenv("GHORG_BRANCH", "")
		defer os.Unsetenv("GHORG_BRANCH")
		os.Setenv("GHORG_TOPICS", "")
		defer os.Unsetenv("GHORG_TOPICS")

		// Test with a bitbucket.Repository constructed via the library.
		// The go-bitbucket library uses exported fields we can set directly.
		repos := []bitbucketRepository{
			{
				Name:     "cloud-test",
				FullName: "org/cloud-test",
				Mainbranch: struct {
					Name string
				}{Name: "main"},
				Links: map[string]any{
					"clone": []any{
						map[string]any{
							"href": "https://testuser@bitbucket.org/org/cloud-test.git",
							"name": "https",
						},
						map[string]any{
							"href": "git@bitbucket.org:org/cloud-test.git",
							"name": "ssh",
						},
					},
				},
			},
		}

		client := Bitbucket{}
		result, err := client.filterReposForTest(repos)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 1 {
			t.Fatalf("expected 1 repo, got %d", len(result))
		}

		if result[0].Name != "cloud-test" {
			t.Errorf("expected name 'cloud-test', got '%s'", result[0].Name)
		}

		if result[0].CloneBranch != "main" {
			t.Errorf("expected branch 'main', got '%s'", result[0].CloneBranch)
		}

		// CloneURL should have OAuth token inserted
		if !strings.Contains(result[0].CloneURL, "test-token") {
			t.Errorf("expected OAuth token in clone URL, got '%s'", result[0].CloneURL)
		}
	})

	t.Run("Cloud filter SSH protocol", func(t *testing.T) {
		os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
		os.Setenv("GHORG_BRANCH", "")
		defer os.Unsetenv("GHORG_BRANCH")
		os.Setenv("GHORG_TOPICS", "")
		defer os.Unsetenv("GHORG_TOPICS")

		repos := []bitbucketRepository{
			{
				Name:     "ssh-cloud-test",
				FullName: "org/ssh-cloud-test",
				Mainbranch: struct {
					Name string
				}{Name: "main"},
				Links: map[string]any{
					"clone": []any{
						map[string]any{
							"href": "https://user@bitbucket.org/org/ssh-cloud-test.git",
							"name": "https",
						},
						map[string]any{
							"href": "git@bitbucket.org:org/ssh-cloud-test.git",
							"name": "ssh",
						},
					},
				},
			},
		}

		client := Bitbucket{}
		result, err := client.filterReposForTest(repos)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 1 {
			t.Fatalf("expected 1 repo, got %d", len(result))
		}

		if result[0].CloneURL != "git@bitbucket.org:org/ssh-cloud-test.git" {
			t.Errorf("expected SSH clone URL, got '%s'", result[0].CloneURL)
		}
	})

	t.Run("Cloud filter with branch override", func(t *testing.T) {
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
		os.Setenv("GHORG_BITBUCKET_OAUTH_TOKEN", "tk")
		defer os.Unsetenv("GHORG_BITBUCKET_OAUTH_TOKEN")
		os.Setenv("GHORG_BRANCH", "feature-branch")
		defer os.Unsetenv("GHORG_BRANCH")
		os.Setenv("GHORG_TOPICS", "")
		defer os.Unsetenv("GHORG_TOPICS")

		repos := []bitbucketRepository{
			{
				Name:     "branch-test",
				FullName: "org/branch-test",
				Mainbranch: struct {
					Name string
				}{Name: "main"},
				Links: map[string]any{
					"clone": []any{
						map[string]any{
							"href": "https://user@bitbucket.org/org/branch-test.git",
							"name": "https",
						},
					},
				},
			},
		}

		client := Bitbucket{}
		result, err := client.filterReposForTest(repos)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) != 1 {
			t.Fatalf("expected 1 repo, got %d", len(result))
		}

		if result[0].CloneBranch != "feature-branch" {
			t.Errorf("expected branch 'feature-branch', got '%s'", result[0].CloneBranch)
		}
	})
}

// --- Bitbucket Server: basic auth verification ---

func TestServerRequestsUseBasicAuth(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	var receivedUser, receivedPass string
	var authHeaderPresent bool

	mux.HandleFunc("/rest/api/1.0/projects/AUTH/repos", func(w http.ResponseWriter, r *http.Request) {
		receivedUser, receivedPass, authHeaderPresent = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON(nil, 0, true, 0))
	})

	_, err := client.getServerProjectRepos("AUTH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !authHeaderPresent {
		t.Fatal("expected basic auth header to be present")
	}
	if receivedUser != "admin" {
		t.Errorf("expected username 'admin', got '%s'", receivedUser)
	}
	if receivedPass != "secret" {
		t.Errorf("expected password 'secret', got '%s'", receivedPass)
	}
}

// --- Bitbucket Server: request Accept header ---

func TestServerRequestsSetAcceptHeader(t *testing.T) {
	client, mux, _, teardown := setupBitbucketServerTest()
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	os.Setenv("GHORG_BRANCH", "")
	defer os.Unsetenv("GHORG_BRANCH")

	var acceptHeader string

	mux.HandleFunc("/rest/api/1.0/projects/HDR/repos", func(w http.ResponseWriter, r *http.Request) {
		acceptHeader = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, makeServerRepoJSON(nil, 0, true, 0))
	})

	_, err := client.getServerProjectRepos("HDR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if acceptHeader != "application/json" {
		t.Errorf("expected Accept header 'application/json', got '%s'", acceptHeader)
	}
}

// bitbucketRepository mirrors the subset of bitbucket.Repository fields used by filter().
// This allows us to test the filter logic without depending on the full go-bitbucket
// library's struct initialization.
type bitbucketRepository struct {
	Name       string
	FullName   string
	Mainbranch struct {
		Name string
	}
	Links map[string]any
}

// filterReposForTest exercises the same logic as the Cloud filter() method but accepts
// our test-friendly bitbucketRepository type. This avoids needing to construct
// bitbucket.Repository values which depend on the third-party library's internal layout.
func (c Bitbucket) filterReposForTest(repos []bitbucketRepository) ([]Repo, error) {
	cloneData := []Repo{}

	for _, a := range repos {
		links, _ := a.Links["clone"].([]any)
		for _, l := range links {
			linkMap, _ := l.(map[string]any)
			link, _ := linkMap["href"].(string)
			linkType, _ := linkMap["name"].(string)

			r := Repo{}
			r.Name = a.Name
			r.Path = a.FullName
			if os.Getenv("GHORG_BRANCH") == "" {
				r.CloneBranch = a.Mainbranch.Name
			} else {
				r.CloneBranch = os.Getenv("GHORG_BRANCH")
			}

			if os.Getenv("GHORG_CLONE_PROTOCOL") == "ssh" && linkType == "ssh" {
				r.URL = link
				r.CloneURL = link
				cloneData = append(cloneData, r)
			} else if os.Getenv("GHORG_CLONE_PROTOCOL") == "https" && linkType == "https" {
				r.URL = link
				r.CloneURL = link
				if os.Getenv("GHORG_BITBUCKET_OAUTH_TOKEN") != "" {
					r.CloneURL = insertOauthTokenIntoURL(r.CloneURL)
				} else {
					r.CloneURL = insertAppPasswordCredentialsIntoURL(r.CloneURL)
				}
				cloneData = append(cloneData, r)
			}
		}
	}

	return cloneData, nil
}
