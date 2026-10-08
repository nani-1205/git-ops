package reporter

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"gitlab-code-scan/internal/services/scanner"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

var projectWorkers = envInt("PROJECT_WORKERS", 4)

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// CommitResult represents a single commit's stats
type CommitResult struct {
	ProjectName   string `json:"project_name"`
	ShortID       string `json:"short_id"`
	Title         string `json:"title"`
	AuthorName    string `json:"author_name"`
	CommittedDate string `json:"committed_date"`
	WebURL        string `json:"web_url"`
	Additions     int    `json:"additions"`
	Deletions     int    `json:"deletions"`
}

// GenerateCommitReport fetches commit statistics across multiple projects
func GenerateCommitReport(projectIDs []int, branch, startDateStr, endDateStr, token string) ([]CommitResult, error) {
	client, err := scanner.GetClient(token)
	if err != nil {
		return nil, err
	}

	layout := "2006-01-02T15:04"
	
	startTime, err := time.Parse(layout, startDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start date format: %v", err)
	}
	
	endTime, err := time.Parse(layout, endDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end date format: %v", err)
	}

	istLoc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		istLoc = time.FixedZone("IST", 5*3600+1800)
	}

	var allCommits []CommitResult
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

				// Check if branch exists
				_, _, err = client.Branches.GetBranch(pid, branch)
				if err != nil {
					log.Printf("⏭️ [REPORTER] Skipping project %s: Branch '%s' not found", proj.PathWithNamespace, branch)
					continue
				}

				since := startTime.UTC()
				until := endTime.UTC()

				opt := &gitlab.ListCommitsOptions{
					RefName: gitlab.Ptr(branch),
					Since:   gitlab.Ptr(since),
					Until:   gitlab.Ptr(until),
					All:     gitlab.Ptr(false),
					ListOptions: gitlab.ListOptions{
						PerPage: 100,
						Page:    1,
					},
				}

				var projCommits []CommitResult

				for {
					commits, resp, err := client.Commits.ListCommits(pid, opt)
					if err != nil {
						log.Printf("⚠️ [REPORTER] Failed to list commits for %s: %v", proj.PathWithNamespace, err)
						break
					}

					for _, c := range commits {
						// Fetch full commit for stats
						fullCommit, _, err := client.Commits.GetCommit(pid, c.ID, nil)
						if err != nil {
							continue
						}

						var commitTime time.Time
						if c.CommittedDate != nil {
							commitTime = *c.CommittedDate
						}
						
						istTime := commitTime.In(istLoc)

						additions := 0
						deletions := 0
						if fullCommit.Stats != nil {
							additions = int(fullCommit.Stats.Additions)
							deletions = int(fullCommit.Stats.Deletions)
						}

						projCommits = append(projCommits, CommitResult{
							ProjectName:   proj.Name,
							ShortID:       c.ShortID,
							Title:         c.Title,
							AuthorName:    c.AuthorName,
							CommittedDate: istTime.Format("2006-01-02 15:04:05"),
							WebURL:        c.WebURL,
							Additions:     additions,
							Deletions:     deletions,
						})
					}

					if resp.CurrentPage >= resp.TotalPages {
						break
					}
					opt.Page = resp.NextPage
				}

				mu.Lock()
				allCommits = append(allCommits, projCommits...)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return allCommits, nil
}
