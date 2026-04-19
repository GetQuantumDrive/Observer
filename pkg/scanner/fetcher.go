package scanner

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FetchRulesFromGitHub downloads YAML rules from a GitHub repository to a local cache directory.
// repoSpec format: "owner/repo", "owner/repo@ref", "owner/repo@ref:subpath",
// or any of the above with "|token" suffix for a per-repo bearer token.
// An inline token (via "|token") takes precedence over the defaultToken argument.
// If ref is omitted, "main" is used. If subpath is set, only files under that directory are extracted.
// Results are cached in ~/.cache/observer/rules/{owner}/{repo}/{ref}/ for 24 hours.
func FetchRulesFromGitHub(repoSpec, defaultToken string) (string, error) {
	spec, inlineToken := splitInlineToken(repoSpec)
	token := inlineToken
	if token == "" {
		token = defaultToken
	}
	owner, repo, ref, subpath := parseRepoSpec(spec)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("invalid rules repo spec %q: expected owner/repo", repoSpec)
	}

	cacheDir := rulesCache(owner, repo, ref)
	if isCacheFresh(cacheDir) {
		if subpath != "" {
			return filepath.Join(cacheDir, subpath), nil
		}
		return cacheDir, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create rules cache dir: %w", err)
	}

	if err := downloadAndExtract(owner, repo, ref, token, cacheDir); err != nil {
		os.RemoveAll(cacheDir)
		return "", fmt.Errorf("fetch %s/%s@%s: %w", owner, repo, ref, err)
	}

	if subpath != "" {
		return filepath.Join(cacheDir, subpath), nil
	}
	return cacheDir, nil
}

// splitInlineToken splits "spec|token" on the last '|'. Tokens containing '|' are not supported.
// Returns (spec, "") if no '|' is present.
func splitInlineToken(spec string) (string, string) {
	i := strings.LastIndex(spec, "|")
	if i < 0 {
		return spec, ""
	}
	return spec[:i], spec[i+1:]
}

// parseRepoSpec splits "owner/repo@ref:subpath" into its components.
func parseRepoSpec(spec string) (owner, repo, ref, subpath string) {
	ref = "main"

	if i := strings.Index(spec, ":"); i >= 0 {
		subpath = spec[i+1:]
		spec = spec[:i]
	}
	if i := strings.Index(spec, "@"); i >= 0 {
		ref = spec[i+1:]
		spec = spec[:i]
	}
	parts := strings.SplitN(spec, "/", 2)
	if len(parts) == 2 {
		owner, repo = parts[0], parts[1]
	}
	return
}

func rulesCache(owner, repo, ref string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "observer", "rules", owner, repo, ref)
}

func isCacheFresh(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < 24*time.Hour
}

func downloadAndExtract(owner, repo, ref, token, destDir string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/zipball/%s", owner, repo, ref)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "observer-rules-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	return extractYAML(tmp.Name(), destDir)
}

// extractYAML extracts only .yaml/.yml files from a GitHub archive zip,
// stripping the top-level "{repo}-{sha}/" prefix GitHub adds to all archives.
func extractYAML(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		// Strip the top-level directory GitHub adds (e.g. "Observer-rules-abc123/")
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		relPath := parts[1]

		dest := filepath.Join(destDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
