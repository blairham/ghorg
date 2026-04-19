package scm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func setupGitlabTest(t *testing.T) (Gitlab, *http.ServeMux, string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	if err != nil {
		t.Fatalf("failed to create gitlab client: %v", err)
	}

	teardown := func() {
		server.Close()
	}

	return Gitlab{Client: client}, mux, server.URL, teardown
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func TestGitlab_GetGroupRepos_SinglePage(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_SKIP_ARCHIVED", "")
	os.Setenv("GHORG_SKIP_FORKS", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_BRANCH", "")
	os.Setenv("GHORG_CLONE_WIKI", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")
	defer os.Unsetenv("GHORG_GITLAB_TOKEN")

	mux.HandleFunc("/api/v4/groups/test-group/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Pages", "1")
		w.Header().Set("X-Page", "1")
		writeJSON(w, []map[string]any{
			{
				"id":                  1,
				"name":                "repo-one",
				"default_branch":      "main",
				"path_with_namespace": "test-group/repo-one",
				"http_url_to_repo":    "https://gitlab.com/test-group/repo-one.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/repo-one.git",
				"archived":            false,
				"topics":              []string{},
				"wiki_access_level":   "disabled",
			},
			{
				"id":                  2,
				"name":                "repo-two",
				"default_branch":      "develop",
				"path_with_namespace": "test-group/repo-two",
				"http_url_to_repo":    "https://gitlab.com/test-group/repo-two.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/repo-two.git",
				"archived":            false,
				"topics":              []string{},
				"wiki_access_level":   "disabled",
			},
		})
	})

	repos, err := client.GetGroupRepos("test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}

	if repos[0].Name != "repo-one" {
		t.Errorf("expected repo name 'repo-one', got %q", repos[0].Name)
	}
	if repos[0].CloneBranch != "main" {
		t.Errorf("expected branch 'main', got %q", repos[0].CloneBranch)
	}
	if repos[1].CloneBranch != "develop" {
		t.Errorf("expected branch 'develop', got %q", repos[1].CloneBranch)
	}
}

func TestGitlab_GetGroupRepos_SkipArchived(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_SKIP_ARCHIVED", "true")
	os.Setenv("GHORG_BRANCH", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_CLONE_WIKI", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")
	defer os.Unsetenv("GHORG_SKIP_ARCHIVED")

	mux.HandleFunc("/api/v4/groups/test-group/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Pages", "1")
		writeJSON(w, []map[string]any{
			{
				"id": 1, "name": "active-repo", "default_branch": "main",
				"path_with_namespace": "test-group/active-repo",
				"http_url_to_repo":    "https://gitlab.com/test-group/active-repo.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/active-repo.git",
				"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
			},
			{
				"id": 2, "name": "archived-repo", "default_branch": "main",
				"path_with_namespace": "test-group/archived-repo",
				"http_url_to_repo":    "https://gitlab.com/test-group/archived-repo.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/archived-repo.git",
				"archived":            true, "topics": []string{}, "wiki_access_level": "disabled",
			},
		})
	})

	repos, err := client.GetGroupRepos("test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (archived skipped), got %d", len(repos))
	}
	if repos[0].Name != "active-repo" {
		t.Errorf("expected 'active-repo', got %q", repos[0].Name)
	}
}

func TestGitlab_GetGroupRepos_SkipForks(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_SKIP_FORKS", "true")
	os.Setenv("GHORG_SKIP_ARCHIVED", "")
	os.Setenv("GHORG_BRANCH", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_CLONE_WIKI", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")
	defer os.Unsetenv("GHORG_SKIP_FORKS")

	mux.HandleFunc("/api/v4/groups/test-group/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Pages", "1")
		writeJSON(w, []map[string]any{
			{
				"id": 1, "name": "original", "default_branch": "main",
				"path_with_namespace": "test-group/original",
				"http_url_to_repo":    "https://gitlab.com/test-group/original.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/original.git",
				"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
			},
			{
				"id": 2, "name": "forked", "default_branch": "main",
				"path_with_namespace": "test-group/forked",
				"http_url_to_repo":    "https://gitlab.com/test-group/forked.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/forked.git",
				"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
				"forked_from_project": map[string]any{"id": 99},
			},
		})
	})

	repos, err := client.GetGroupRepos("test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (fork skipped), got %d", len(repos))
	}
	if repos[0].Name != "original" {
		t.Errorf("expected 'original', got %q", repos[0].Name)
	}
}

func TestGitlab_GetGroupRepos_MultiPage(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_SKIP_ARCHIVED", "")
	os.Setenv("GHORG_SKIP_FORKS", "")
	os.Setenv("GHORG_BRANCH", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_CLONE_WIKI", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")

	mux.HandleFunc("/api/v4/groups/test-group/projects", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("X-Total-Pages", "2")

		if page == "" || page == "1" {
			w.Header().Set("X-Page", "1")
			writeJSON(w, []map[string]any{
				{
					"id": 1, "name": "page1-repo", "default_branch": "main",
					"path_with_namespace": "test-group/page1-repo",
					"http_url_to_repo":    "https://gitlab.com/test-group/page1-repo.git",
					"ssh_url_to_repo":     "git@gitlab.com:test-group/page1-repo.git",
					"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
				},
			})
		} else {
			w.Header().Set("X-Page", "2")
			writeJSON(w, []map[string]any{
				{
					"id": 2, "name": "page2-repo", "default_branch": "main",
					"path_with_namespace": "test-group/page2-repo",
					"http_url_to_repo":    "https://gitlab.com/test-group/page2-repo.git",
					"ssh_url_to_repo":     "git@gitlab.com:test-group/page2-repo.git",
					"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
				},
			})
		}
	})

	repos, err := client.GetGroupRepos("test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos across 2 pages, got %d", len(repos))
	}

	names := map[string]bool{}
	for _, r := range repos {
		names[r.Name] = true
	}
	if !names["page1-repo"] || !names["page2-repo"] {
		t.Errorf("expected both page1-repo and page2-repo, got %v", names)
	}
}

func TestGitlab_GetGroupRepos_SSHProtocol(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_SKIP_ARCHIVED", "")
	os.Setenv("GHORG_SKIP_FORKS", "")
	os.Setenv("GHORG_BRANCH", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_CLONE_WIKI", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")
	defer os.Unsetenv("GHORG_CLONE_PROTOCOL")

	mux.HandleFunc("/api/v4/groups/test-group/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Pages", "1")
		writeJSON(w, []map[string]any{
			{
				"id": 1, "name": "ssh-repo", "default_branch": "main",
				"path_with_namespace": "test-group/ssh-repo",
				"http_url_to_repo":    "https://gitlab.com/test-group/ssh-repo.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/ssh-repo.git",
				"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
			},
		})
	})

	repos, err := client.GetGroupRepos("test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repos[0].CloneURL != "git@gitlab.com:test-group/ssh-repo.git" {
		t.Errorf("expected SSH clone URL, got %q", repos[0].CloneURL)
	}
}

func TestGitlab_GetGroupRepos_404(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")

	mux.HandleFunc("/api/v4/groups/nonexistent/projects", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"message": "404 Group Not Found"})
	})

	_, err := client.GetGroupRepos("nonexistent")
	if err == nil {
		t.Fatal("expected error for 404 group, got nil")
	}
}

func TestGitlab_SnippetCloneURL(t *testing.T) {
	client := Gitlab{}

	t.Run("https snippet URL", func(t *testing.T) {
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		os.Setenv("GHORG_INSECURE_GITLAB_CLIENT", "")
		got := client.createRepoSnippetCloneURL("https://gitlab.com/group/repo.git", "123")
		want := "https://gitlab.com/group/repo/snippets/123.git"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("ssh snippet URL from https input", func(t *testing.T) {
		// The function expects HTTPS clone URLs as input and converts to SSH when protocol is ssh
		os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
		os.Setenv("GHORG_INSECURE_GITLAB_CLIENT", "")
		got := client.createRepoSnippetCloneURL("https://gitlab.com/group/repo.git", "456")
		want := "git@gitlab.com:group/repo/snippets/456.git"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestGitlab_RootLevelSnippetCloneURL(t *testing.T) {
	client := Gitlab{}

	t.Run("https root snippet", func(t *testing.T) {
		os.Setenv("GHORG_CLONE_PROTOCOL", "https")
		os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
		os.Setenv("GHORG_INSECURE_GITLAB_CLIENT", "")
		got := client.createRootLevelSnippetCloneURL("https://gitlab.com/-/snippets/100")
		want := "https://oauth2:test-token@gitlab.com/snippets/100.git"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("ssh root snippet", func(t *testing.T) {
		os.Setenv("GHORG_CLONE_PROTOCOL", "ssh")
		got := client.createRootLevelSnippetCloneURL("https://gitlab.com/-/snippets/100")
		want := "git@gitlab.com:snippets/100.git"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestGitlab_RootLevelSnippetDetection(t *testing.T) {
	client := Gitlab{}

	t.Run("cloud root snippet", func(t *testing.T) {
		os.Setenv("GHORG_SCM_BASE_URL", "")
		if !client.rootLevelSnippet("https://gitlab.com/-/snippets/123") {
			t.Error("expected root level snippet to be detected")
		}
	})

	t.Run("cloud non-root snippet", func(t *testing.T) {
		os.Setenv("GHORG_SCM_BASE_URL", "")
		if client.rootLevelSnippet("https://gitlab.com/group/project/-/snippets/123") {
			t.Error("expected non-root snippet to NOT be detected as root")
		}
	})

	t.Run("self-hosted root snippet", func(t *testing.T) {
		os.Setenv("GHORG_SCM_BASE_URL", "https://git.example.com")
		defer os.Unsetenv("GHORG_SCM_BASE_URL")
		if !client.rootLevelSnippet("https://git.example.com/-/snippets/5") {
			t.Error("expected self-hosted root snippet to be detected")
		}
	})
}

func TestGitlab_FilterGroupByExcludeRegex(t *testing.T) {
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "^internal-")
	defer os.Unsetenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX")

	groups := []string{"public-group", "internal-tools", "internal-infra", "open-source"}
	filtered := filterGitlabGroupByExcludeMatchRegex(groups)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 groups after filtering, got %d: %v", len(filtered), filtered)
	}
	if filtered[0] != "public-group" || filtered[1] != "open-source" {
		t.Errorf("unexpected filtered groups: %v", filtered)
	}
}

func TestGitlab_ToSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"My Snippet Title!", "my-snippet-title"},
		{"  spaces  ", "spaces"},
		{"CamelCase", "camelcase"},
		{"special@#chars", "special-chars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := ToSlug(tt.input)
			if got != tt.want {
				t.Errorf("ToSlug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGitlab_AddTokenToCloneURL(t *testing.T) {
	t.Parallel()
	client := Gitlab{}
	got := client.addTokenToCloneURL("https://gitlab.com/group/repo.git", "mytoken")
	want := "https://oauth2:mytoken@gitlab.com/group/repo.git"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGitlab_GetGroupRepos_WikiClone(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_CLONE_WIKI", "true")
	os.Setenv("GHORG_SKIP_ARCHIVED", "")
	os.Setenv("GHORG_SKIP_FORKS", "")
	os.Setenv("GHORG_BRANCH", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")
	defer os.Unsetenv("GHORG_CLONE_WIKI")

	mux.HandleFunc("/api/v4/groups/test-group/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Pages", "1")
		writeJSON(w, []map[string]any{
			{
				"id": 1, "name": "wiki-repo", "default_branch": "main",
				"path_with_namespace": "test-group/wiki-repo",
				"http_url_to_repo":    "https://gitlab.com/test-group/wiki-repo.git",
				"ssh_url_to_repo":     "git@gitlab.com:test-group/wiki-repo.git",
				"archived":            false, "topics": []string{},
				"wiki_access_level": "enabled",
			},
		})
	})

	repos, err := client.GetGroupRepos("test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 entries (repo + wiki), got %d", len(repos))
	}

	wikiFound := false
	for _, r := range repos {
		if r.IsWiki {
			wikiFound = true
			if r.CloneBranch != "master" {
				t.Errorf("wiki branch should be 'master', got %q", r.CloneBranch)
			}
			if !contains(r.CloneURL, ".wiki.git") {
				t.Errorf("wiki clone URL should contain .wiki.git, got %q", r.CloneURL)
			}
		}
	}
	if !wikiFound {
		t.Error("expected wiki repo to be included")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGitlab_GetTopLevelGroups_SinglePage(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	mux.HandleFunc("/api/v4/groups", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Pages", "1")
		writeJSON(w, []map[string]any{
			{"id": 10, "name": "group-a"},
			{"id": 20, "name": "group-b"},
		})
	})

	groups, err := client.GetTopLevelGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0] != "10" || groups[1] != "20" {
		t.Errorf("expected group IDs [10 20], got %v", groups)
	}
}

func TestGitlab_GetTopLevelGroups_MultiPage(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	mux.HandleFunc("/api/v4/groups", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("X-Total-Pages", "2")
		if page == "" || page == "1" {
			writeJSON(w, []map[string]any{{"id": 10, "name": "group-a"}})
		} else {
			writeJSON(w, []map[string]any{{"id": 20, "name": "group-b"}})
		}
	})

	groups, err := client.GetTopLevelGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups across pages, got %d", len(groups))
	}
}

func TestGitlab_GetSnippets_Disabled(t *testing.T) {
	client := Gitlab{}
	os.Setenv("GHORG_CLONE_SNIPPETS", "")
	defer os.Unsetenv("GHORG_CLONE_SNIPPETS")

	snippets, err := client.GetSnippets([]Repo{}, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snippets) != 0 {
		t.Errorf("expected no snippets when disabled, got %d", len(snippets))
	}
}

func TestGitlab_GetSnippets_CloudGroup(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_SNIPPETS", "true")
	os.Setenv("GHORG_CLONE_TYPE", "org")
	os.Setenv("GHORG_SCM_BASE_URL", "")
	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	defer os.Unsetenv("GHORG_CLONE_SNIPPETS")
	defer os.Unsetenv("GHORG_CLONE_TYPE")

	// Mock project snippets endpoint
	mux.HandleFunc("/api/v4/projects/1/snippets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{
				"id": 100, "title": "My Snippet", "project_id": 1,
				"web_url": "https://gitlab.com/test-group/repo-one/-/snippets/100",
			},
		})
	})
	mux.HandleFunc("/api/v4/projects/2/snippets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{})
	})

	cloneData := []Repo{
		{ID: "1", Name: "repo-one", CloneURL: "https://gitlab.com/test-group/repo-one.git", URL: "https://gitlab.com/test-group/repo-one.git", Path: "/repo-one"},
		{ID: "2", Name: "repo-two", CloneURL: "https://gitlab.com/test-group/repo-two.git", URL: "https://gitlab.com/test-group/repo-two.git", Path: "/repo-two"},
	}

	snippets, err := client.GetSnippets(cloneData, "test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(snippets))
	}

	if !snippets[0].IsGitLabSnippet {
		t.Error("expected snippet flag to be set")
	}
	if snippets[0].GitLabSnippetInfo.ID != "100" {
		t.Errorf("expected snippet ID '100', got %q", snippets[0].GitLabSnippetInfo.ID)
	}
}

func TestGitlab_BranchOverride(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_BRANCH", "custom-branch")
	os.Setenv("GHORG_SKIP_ARCHIVED", "")
	os.Setenv("GHORG_SKIP_FORKS", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_CLONE_WIKI", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")
	defer os.Unsetenv("GHORG_BRANCH")

	mux.HandleFunc("/api/v4/groups/test-group/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Pages", "1")
		writeJSON(w, []map[string]any{
			{
				"id": 1, "name": "repo", "default_branch": "main",
				"path_with_namespace": "test-group/repo",
				"http_url_to_repo":    fmt.Sprintf("%s/test-group/repo.git", "https://gitlab.com"),
				"ssh_url_to_repo":     "git@gitlab.com:test-group/repo.git",
				"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
			},
		})
	})

	repos, err := client.GetGroupRepos("test-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repos[0].CloneBranch != "custom-branch" {
		t.Errorf("expected 'custom-branch', got %q", repos[0].CloneBranch)
	}
}

func TestGitlab_GetGroupRepos_MultiPage_ErrorOnPage(t *testing.T) {
	client, mux, _, teardown := setupGitlabTest(t)
	defer teardown()

	os.Setenv("GHORG_CLONE_PROTOCOL", "https")
	os.Setenv("GHORG_GITLAB_TOKEN", "test-token")
	os.Setenv("GHORG_SKIP_ARCHIVED", "")
	os.Setenv("GHORG_SKIP_FORKS", "")
	os.Setenv("GHORG_BRANCH", "")
	os.Setenv("GHORG_TOPICS", "")
	os.Setenv("GHORG_CLONE_WIKI", "")
	os.Setenv("GHORG_GITLAB_GROUP_EXCLUDE_MATCH_REGEX", "")

	mux.HandleFunc("/api/v4/groups/error-group/projects", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("X-Total-Pages", "3")

		switch page {
		case "", "1":
			w.Header().Set("X-Page", "1")
			writeJSON(w, []map[string]any{
				{
					"id": 1, "name": "repo1", "default_branch": "main",
					"path_with_namespace": "error-group/repo1",
					"http_url_to_repo":    "https://gitlab.com/error-group/repo1.git",
					"ssh_url_to_repo":     "git@gitlab.com:error-group/repo1.git",
					"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
				},
			})
		case "2":
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		case "3":
			w.Header().Set("X-Page", "3")
			writeJSON(w, []map[string]any{
				{
					"id": 3, "name": "repo3", "default_branch": "main",
					"path_with_namespace": "error-group/repo3",
					"http_url_to_repo":    "https://gitlab.com/error-group/repo3.git",
					"ssh_url_to_repo":     "git@gitlab.com:error-group/repo3.git",
					"archived":            false, "topics": []string{}, "wiki_access_level": "disabled",
				},
			})
		}
	})

	_, err := client.GetGroupRepos("error-group")
	if err == nil {
		t.Fatal("expected error when page 2 returns 500, got nil")
	}
}
