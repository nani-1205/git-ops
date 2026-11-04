package reporter

import (
	"log"
	"strings"
	"sync"

	"gitlab-code-scan/internal/services/scanner"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// CompareResult represents a branch comparison between source and target
type CompareResult struct {
	ProjectName   string `json:"project_name"`
	SourceBranch  string `json:"source_branch"`
	TargetBranch  string `json:"target_branch"`
	Ahead         int    `json:"ahead"`
	Behind        int    `json:"behind"`
	FilesChanged  int    `json:"files_changed"`
	Additions     int    `json:"additions"`
	Deletions     int    `json:"deletions"`
	Status        string `json:"status"`
	ModifiedFiles string `json:"modified_files"`
}

// CompareBranches compares two branches across multiple projects
func CompareBranches(projectIDs []int, sourceBranch, targetBranch, token string) ([]CompareResult, error) {
	client, err := scanner.GetClient(token)
	if err != nil {
		return nil, err
	}

	var allComparisons []CompareResult
	var mu sync.Mutex

	projectsCh := make(chan int, len(projectIDs))
	for _, pid := range projectIDs {
		projectsCh <- pid
	}
	close(projectsCh)

	var wg sync.WaitGroup
	workers := projectWorkers
	if len(projectIDs) < workers {
		workers = len(projectIDs)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pid := range projectsCh {
				proj, _, err := client.Projects.GetProject(pid, nil)
				if err != nil {
					log.Printf("⚠️ [REPORTER] Failed to get project %d: %v", pid, err)
					continue
				}

				// Check if both branches exist
				_, _, err1 := client.Branches.GetBranch(pid, sourceBranch)
				_, _, err2 := client.Branches.GetBranch(pid, targetBranch)
				if err1 != nil || err2 != nil {
					log.Printf("⏭️ [REPORTER] Skipping project %s: One or both branches not found", proj.PathWithNamespace)
					mu.Lock()
					allComparisons = append(allComparisons, CompareResult{
						ProjectName:  proj.PathWithNamespace,
						SourceBranch: sourceBranch,
						TargetBranch: targetBranch,
						Status:       "Branch missing",
					})
					mu.Unlock()
					continue
				}

				// Compare target to source (Ahead changes)
				optAhead := &gitlab.CompareOptions{
					From: gitlab.Ptr(targetBranch),
					To:   gitlab.Ptr(sourceBranch),
				}
				
				compareAhead, _, err := client.Repositories.Compare(pid, optAhead)
				if err != nil {
					log.Printf("⚠️ [REPORTER] Failed to compare ahead for %s: %v", proj.PathWithNamespace, err)
					continue
				}

				aheadCount := len(compareAhead.Commits)
				changedFiles := len(compareAhead.Diffs)
				additions := 0
				deletions := 0
				var fileNames []string

				for _, diff := range compareAhead.Diffs {
					path := diff.NewPath
					if path == "" {
						path = diff.OldPath
					}
					if path != "" {
						fileNames = append(fileNames, path)
					}

					// Count additions and deletions manually from the diff string
					lines := strings.Split(diff.Diff, "\n")
					for _, line := range lines {
						if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++ ") {
							additions++
						} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "--- ") {
							deletions++
						}
					}
				}

				// Compare source to target (Behind changes)
				optBehind := &gitlab.CompareOptions{
					From: gitlab.Ptr(sourceBranch),
					To:   gitlab.Ptr(targetBranch),
				}

				compareBehind, _, err := client.Repositories.Compare(pid, optBehind)
				behindCount := 0
				if err == nil && compareBehind != nil {
					behindCount = len(compareBehind.Commits)
				}

				status := "Up to date"
				if aheadCount > 0 && behindCount > 0 {
					status = "Diverged"
				} else if aheadCount > 0 {
					status = "Ahead"
				} else if behindCount > 0 {
					status = "Behind"
				}

				mu.Lock()
				allComparisons = append(allComparisons, CompareResult{
					ProjectName:   proj.PathWithNamespace,
					SourceBranch:  sourceBranch,
					TargetBranch:  targetBranch,
					Ahead:         aheadCount,
					Behind:        behindCount,
					FilesChanged:  changedFiles,
					Additions:     additions,
					Deletions:     deletions,
					Status:        status,
					ModifiedFiles: strings.Join(fileNames, ", "),
				})
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return allComparisons, nil
}
