package scanner

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"gitlab-code-scan/internal/config"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// ---------------------------------------------------------------------------
// Tunables – override via environment variables (same pattern as old Python).
// PROJECT_WORKERS: number of repos scanned concurrently         (default 4)
// FILE_WORKERS   : number of files fetched per repo concurrently (default 3)
// ---------------------------------------------------------------------------

var (
	projectWorkers = envInt("PROJECT_WORKERS", 4)
	fileWorkers    = envInt("FILE_WORKERS", 3)
)

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

// ScanResult holds one keyword match inside a file.
type ScanResult struct {
	ProjectID    int64  `json:"project_id"`
	ProjectName  string `json:"project_name"`
	ProjectURL   string `json:"project_url"`
	Filename     string `json:"filename"`
	LineContent  string `json:"line_content"`
	LineNumber   int    `json:"line_number"`
	DeepLink     string `json:"deep_link"`
	KeywordFound string `json:"keyword_found"`
	Branch       string `json:"branch"`
}

// lineMatch is the internal match type used by extractMatches.
type lineMatch struct {
	term    string
	lineNo  int
	content string
}

// ScanProgress is safe for concurrent reads from the status endpoint.
type ScanProgress struct {
	ProjectsTotal   int64 `json:"projects_total"`
	ProjectsScanned int64 `json:"projects_scanned"`
	TotalMatches    int64 `json:"total_matches"`
}

// ---------------------------------------------------------------------------
// GitLab client
// ---------------------------------------------------------------------------

// GetClient creates an authenticated GitLab client using the provided OAuth token.
func GetClient(token string) (*gitlab.Client, error) {
	return gitlab.NewOAuthClient(
		token,
		gitlab.WithBaseURL(config.GitLabURL+"/api/v4"),
		gitlab.WithHTTPClient(config.InsecureHTTPClient),
	)
}

// ---------------------------------------------------------------------------
// Group traversal  (BFS – mirrors Python's collect_group_projects)
// ---------------------------------------------------------------------------

// collectGroupProjects returns every project reachable from rootGroupID via BFS
// over subgroups, deduplicating by project ID.
func collectGroupProjects(client *gitlab.Client, rootGroupID string) ([]*gitlab.Project, int, error) {
	type queueItem struct {
		id interface{} // int or string on first call
	}

	queue := []queueItem{{id: rootGroupID}}
	visitedGroups := map[int64]bool{}
	projectsByID := map[int64]*gitlab.Project{}
	groupCount := 0

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		var group *gitlab.Group
		var err error
		switch v := cur.id.(type) {
		case int:
			group, _, err = client.Groups.GetGroup(v, nil)
		case string:
			group, _, err = client.Groups.GetGroup(v, nil)
		}
		if err != nil {
			log.Printf("⚠️  [BFS] Could not fetch group %v: %v", cur.id, err)
			continue
		}
		if visitedGroups[int64(group.ID)] {
			continue
		}
		visitedGroups[int64(group.ID)] = true
		groupCount++
		log.Printf("📂 [BFS] Visiting group: %s (id=%d)", group.FullPath, group.ID)

		// Collect projects in this group.
		pOpt := &gitlab.ListGroupProjectsOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
		}
		for {
			projects, resp, pErr := client.Groups.ListGroupProjects(group.ID, pOpt)
			if pErr != nil {
				log.Printf("⚠️  [BFS] Failed to list projects for group %s: %v", group.FullPath, pErr)
				break
			}
			for _, p := range projects {
				projectsByID[int64(p.ID)] = p
			}
			if resp.CurrentPage >= resp.TotalPages {
				break
			}
			pOpt.Page = resp.NextPage
		}

		// Enqueue subgroups.
		sgOpt := &gitlab.ListSubGroupsOptions{
			ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
		}
		for {
			subgroups, resp, sgErr := client.Groups.ListSubGroups(group.ID, sgOpt)
			if sgErr != nil {
				break
			}
			for _, sg := range subgroups {
				if !visitedGroups[int64(sg.ID)] {
					log.Printf("🔍 [BFS] Queuing subgroup: %s", sg.FullPath)
					queue = append(queue, queueItem{id: sg.ID})
				}
			}
			if resp.CurrentPage >= resp.TotalPages {
				break
			}
			sgOpt.Page = resp.NextPage
		}
	}
	log.Printf("✅ [BFS] Done — %d group(s) visited, %d project(s) collected", groupCount, len(projectsByID))

	projects := make([]*gitlab.Project, 0, len(projectsByID))
	for _, p := range projectsByID {
		projects = append(projects, p)
	}
	return projects, groupCount, nil
}

// ---------------------------------------------------------------------------
// File-level helpers
// ---------------------------------------------------------------------------

// resolveRef returns the target branch if it exists, otherwise the project's
// default branch – mirrors Python's resolve_search_ref.
func resolveRef(client *gitlab.Client, projectID int64, target string) string {
	if target != "" {
		_, _, err := client.Branches.GetBranch(int(projectID), target)
		if err == nil {
			return target
		}
	}
	proj, _, err := client.Projects.GetProject(int(projectID), nil)
	if err != nil || proj.DefaultBranch == "" {
		return "main"
	}
	return proj.DefaultBranch
}

// buildDeepLink constructs a GitLab blob URL with a line anchor.
func buildDeepLink(webURL, ref, filePath string, lineNo int) string {
	if webURL == "" || ref == "" || filePath == "" {
		return ""
	}
	encodedRef := url.PathEscape(ref)
	parts := strings.Split(filePath, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return fmt.Sprintf("%s/-/blob/%s/%s#L%d", webURL, encodedRef, strings.Join(parts, "/"), lineNo)
}

// fetchFileContent fetches raw bytes for a file.
// Primary : Files API (fast, rejects files > ~1 MB).
// Fallback: repository blob by SHA (works for any size).
func fetchFileContent(client *gitlab.Client, projectID int64, filePath, blobSHA, ref string) ([]byte, error) {
	f, _, err := client.RepositoryFiles.GetFile(int(projectID), filePath, &gitlab.GetFileOptions{Ref: gitlab.Ptr(ref)})
	if err == nil {
		decoded, decErr := base64.StdEncoding.DecodeString(f.Content)
		if decErr == nil {
			return decoded, nil
		}
		return []byte(f.Content), nil
	}
	if blobSHA == "" {
		return nil, fmt.Errorf("files API failed and no blob SHA: %w", err)
	}
	blob, _, bErr := client.Repositories.RawBlobContent(int(projectID), blobSHA)
	if bErr != nil {
		return nil, fmt.Errorf("files API and blob fallback both failed: %w", bErr)
	}
	return blob, nil
}

// extractMatches scans text line-by-line for all terms (case-insensitive).
// Mirrors Python's extract_case_insensitive_matches.
func extractMatches(text string, terms []string) []lineMatch {
	var results []lineMatch
	seen := map[string]bool{}

	lowered := make([]string, len(terms))
	for i, t := range terms {
		lowered[i] = strings.ToLower(t)
	}

	for lineNo, line := range strings.Split(text, "\n") {
		ll := strings.ToLower(line)
		for i, lt := range lowered {
			if lt != "" && strings.Contains(ll, lt) {
				snippet := strings.TrimSpace(line)
				key := fmt.Sprintf("%s\x00%d\x00%s", terms[i], lineNo+1, snippet)
				if !seen[key] {
					seen[key] = true
					results = append(results, lineMatch{term: terms[i], lineNo: lineNo + 1, content: snippet})
				}
			}
		}
	}
	return results
}

// scanSingleFile fetches and scans one file, returning all keyword matches.
func scanSingleFile(
	client *gitlab.Client,
	project *gitlab.Project,
	filePath, blobSHA, ref string,
	terms []string,
) []ScanResult {
	content, err := fetchFileContent(client, project.ID, filePath, blobSHA, ref)
	if err != nil {
		log.Printf("⚠️  [FILE] Skip %s/%s — fetch error: %v", project.PathWithNamespace, filePath, err)
		return nil
	}

	text := string(content)

	// Skip binary: check first 8 KB for null bytes.
	limit := len(text)
	if limit > 8192 {
		limit = 8192
	}
	if strings.ContainsRune(text[:limit], 0) {
		log.Printf("⏭️  [FILE] Skip binary file: %s/%s", project.PathWithNamespace, filePath)
		return nil
	}

	matches := extractMatches(text, terms)
	results := make([]ScanResult, 0, len(matches))
	for _, m := range matches {
		results = append(results, ScanResult{
			ProjectID:    int64(project.ID),
			ProjectName:  project.NameWithNamespace,
			ProjectURL:   project.WebURL,
			Filename:     filePath,
			LineContent:  m.content,
			LineNumber:   m.lineNo,
			DeepLink:     buildDeepLink(project.WebURL, ref, filePath, m.lineNo),
			KeywordFound: m.term,
			Branch:       ref,
		})
	}
	return results
}

// ---------------------------------------------------------------------------
// Project-level scan (called from the project worker pool)
// ---------------------------------------------------------------------------

func scanSingleProject(
	client *gitlab.Client,
	projectID int,
	terms []string,
	targetBranch string,
	progress *ScanProgress,
) []ScanResult {
	project, _, err := client.Projects.GetProject(projectID, nil)
	if err != nil {
		log.Printf("⚠️  [PROJECT] Cannot fetch project id=%d: %v", projectID, err)
		atomic.AddInt64(&progress.ProjectsScanned, 1)
		return nil
	}

	ref := resolveRef(client, int64(project.ID), targetBranch)
	scanned := atomic.LoadInt64(&progress.ProjectsScanned)
	total := atomic.LoadInt64(&progress.ProjectsTotal)
	log.Printf("🚀 [PROJECT] Starting (%d/%d) %s | branch=%s",
		scanned+1, total, project.PathWithNamespace, ref)

	// Walk the full repository tree recursively.
	treeOpt := &gitlab.ListTreeOptions{
		Ref:       gitlab.Ptr(ref),
		Recursive: gitlab.Ptr(true),
		ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
	}
	type blobEntry struct{ path, sha string }
	var blobs []blobEntry

	for {
		nodes, resp, tErr := client.Repositories.ListTree(project.ID, treeOpt)
		if tErr != nil {
			log.Printf("⚠️  [PROJECT] Cannot list tree for %s: %v", project.PathWithNamespace, tErr)
			break
		}
		for _, n := range nodes {
			if n.Type == "blob" {
				blobs = append(blobs, blobEntry{path: n.Path, sha: n.ID})
			}
		}
		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		treeOpt.Page = resp.NextPage
	}
	log.Printf("📋 [PROJECT] %s — %d file(s) to scan with %d worker(s)",
		project.PathWithNamespace, len(blobs), fileWorkers)

	// Scan files concurrently with FILE_WORKERS goroutines.
	type job struct{ path, sha string }
	jobs := make(chan job, len(blobs))
	for _, b := range blobs {
		jobs <- job{b.path, b.sha}
	}
	close(jobs)

	var mu sync.Mutex
	var allResults []ScanResult
	var wg sync.WaitGroup

	for w := 0; w < fileWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				res := scanSingleFile(client, project, j.path, j.sha, ref, terms)
				if len(res) > 0 {
					mu.Lock()
					allResults = append(allResults, res...)
					mu.Unlock()
					atomic.AddInt64(&progress.TotalMatches, int64(len(res)))
					for _, r := range res {
						log.Printf("🎯 [MATCH] keyword=%q file=%s line=%d project=%s",
							r.KeywordFound, r.Filename, r.LineNumber, project.PathWithNamespace)
					}
				}
			}
		}()
	}
	wg.Wait()

	atomic.AddInt64(&progress.ProjectsScanned, 1)
	doneSoFar := atomic.LoadInt64(&progress.ProjectsScanned)
	totalMatches := atomic.LoadInt64(&progress.TotalMatches)
	log.Printf("✅ [PROJECT] Done (%d/%d) %s | files=%d | matches=%d | total_matches_so_far=%d",
		doneSoFar, total, project.PathWithNamespace, len(blobs), len(allResults), totalMatches)
	return allResults
}

// ---------------------------------------------------------------------------
// Public entry points
// ---------------------------------------------------------------------------

// ScanProjects scans an explicit list of project IDs using two-level parallelism.
func ScanProjects(projectIDs []int, keywords []string, branch string, token string) ([]ScanResult, error) {
	results, _, err := ScanProjectsWithProgress(projectIDs, keywords, branch, token, nil)
	return results, err
}

// ScanProjectsWithProgress is like ScanProjects but accepts a *ScanProgress for
// live status reporting. Pass nil to allocate a fresh one.
func ScanProjectsWithProgress(
	projectIDs []int,
	keywords []string,
	branch string,
	token string,
	progress *ScanProgress,
) ([]ScanResult, *ScanProgress, error) {
	client, err := GetClient(token)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	if progress == nil {
		progress = &ScanProgress{}
	}
	atomic.StoreInt64(&progress.ProjectsTotal, int64(len(projectIDs)))

	terms := dedup(keywords)

	jobs := make(chan int, len(projectIDs))
	for _, id := range projectIDs {
		jobs <- id
	}
	close(jobs)

	var mu sync.Mutex
	var allResults []ScanResult
	var wg sync.WaitGroup

	log.Printf("🔎 [SCAN] Starting — %d project(s) | %d project-worker(s) × %d file-worker(s) | terms=%v",
		len(projectIDs), projectWorkers, fileWorkers, terms)

	for w := 0; w < projectWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pid := range jobs {
				res := scanSingleProject(client, pid, terms, branch, progress)
				if len(res) > 0 {
					mu.Lock()
					allResults = append(allResults, res...)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	log.Printf("🏁 [SCAN] Completed — %d project(s) scanned | total matches=%d",
		atomic.LoadInt64(&progress.ProjectsScanned),
		atomic.LoadInt64(&progress.TotalMatches))
	return allResults, progress, nil
}

// ScanGroup collects all projects under a group hierarchy via BFS then scans them.
func ScanGroup(groupID string, keywords []string, branch string, token string) ([]ScanResult, error) {
	client, err := GetClient(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %w", err)
	}

	projects, groupCount, err := collectGroupProjects(client, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to collect projects: %w", err)
	}

	log.Printf("[INFO] ScanGroup %s: %d project(s) across %d group(s)",
		groupID, len(projects), groupCount)

	ids := make([]int, len(projects))
	for i, p := range projects {
		ids[i] = int(p.ID)
	}

	results, _, err := ScanProjectsWithProgress(ids, keywords, branch, token, nil)
	return results, err
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func dedup(ss []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
