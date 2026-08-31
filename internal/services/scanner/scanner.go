package scanner

import (
	"fmt"
	"strings"
	"sync"

	"gitlab-code-scan/internal/config"

	"gitlab.com/gitlab-org/api/client-go"
)

type ScanResult struct {
	ProjectID    int      `json:"project_id"`
	ProjectName  string   `json:"project_name"`
	ProjectURL   string   `json:"project_url"`
	Filename     string   `json:"filename"`
	LineContent  string   `json:"line_content"`
	LineNumber   int      `json:"line_number"` // if available, or approximate
	DeepLink     string   `json:"deep_link"`
	KeywordFound string   `json:"keyword_found"`
}

func GetClient(token string) (*gitlab.Client, error) {
	return gitlab.NewOAuthClient(token, gitlab.WithBaseURL(config.GitLabURL+"/api/v4"), gitlab.WithHTTPClient(config.InsecureHTTPClient))
}

func ScanGroup(groupID string, keywords []string, branch string, token string) ([]ScanResult, error) {
	client, err := GetClient(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %v", err)
	}

	var allResults []ScanResult

	// Get all projects in the group (and subgroups)
	opt := &gitlab.ListGroupProjectsOptions{
		IncludeSubGroups: gitlab.Ptr(true),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	for {
		projects, resp, err := client.Groups.ListGroupProjects(groupID, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list group projects: %v", err)
		}

		for _, project := range projects {
			for _, keyword := range keywords {
				// We can use the Advanced Search API (requires Elasticsearch enabled on GitLab)
				// Or the Blob search API per project
				
				searchOpt := &gitlab.SearchOptions{
					ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
				}
				if branch != "" {
					searchOpt.Ref = gitlab.Ptr(branch)
				}
				
				// Keep paginating search results for this project/keyword
				for {
					blobs, searchResp, err := client.Search.BlobsByProject(project.ID, keyword, searchOpt)
					if err != nil {
						// Some projects might return 403 or 400 if empty, just skip on error
						break
					}

					for _, blob := range blobs {
						// The search API returns the file and some context lines (highlighting)
						// Go-gitlab `Blob` struct has Data (the content snippet)
						
						// Example Deep Link: https://gitlab.com/group/project/-/blob/master/path/to/file#L123
						deepLink := fmt.Sprintf("%s/-/blob/%s/%s#L%d", project.WebURL, blob.Ref, blob.Path, blob.Startline)
						
						result := ScanResult{
							ProjectID:    int(project.ID),
							ProjectName:  project.NameWithNamespace,
							ProjectURL:   project.WebURL,
							Filename:     blob.Path,
							LineContent:  strings.TrimSpace(blob.Data), // Data usually contains the snippet
							LineNumber:   int(blob.Startline), 
							DeepLink:     deepLink,
							KeywordFound: keyword,
						}
						allResults = append(allResults, result)
					}
					
					if searchResp.CurrentPage >= searchResp.TotalPages {
						break
					}
					searchOpt.Page = searchResp.NextPage
				}
			}
		}

		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		opt.Page = resp.NextPage
	}

	return allResults, nil
}

func ScanProjects(projectIDs []int, keywords []string, branch string, token string) ([]ScanResult, error) {
	client, err := GetClient(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitlab client: %v", err)
	}

	var allResults []ScanResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to avoid hitting rate limits or overwhelming the server
	sem := make(chan struct{}, 15)

	for _, pid := range projectIDs {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Get project to get web URL and namespace
			project, _, err := client.Projects.GetProject(pid, nil)
			if err != nil {
				// skip projects we can't fetch
				return
			}

			var localResults []ScanResult
			for _, keyword := range keywords {
				searchOpt := &gitlab.SearchOptions{
					ListOptions: gitlab.ListOptions{PerPage: 100, Page: 1},
				}
				if branch != "" {
					searchOpt.Ref = gitlab.Ptr(branch)
				}

				for {
					blobs, searchResp, err := client.Search.BlobsByProject(project.ID, keyword, searchOpt)
					if err != nil {
						break
					}

					for _, blob := range blobs {
						deepLink := fmt.Sprintf("%s/-/blob/%s/%s#L%d", project.WebURL, blob.Ref, blob.Path, blob.Startline)
						
						result := ScanResult{
							ProjectID:    int(project.ID),
							ProjectName:  project.NameWithNamespace,
							ProjectURL:   project.WebURL,
							Filename:     blob.Path,
							LineContent:  strings.TrimSpace(blob.Data),
							LineNumber:   int(blob.Startline), 
							DeepLink:     deepLink,
							KeywordFound: keyword,
						}
						localResults = append(localResults, result)
					}
					
					if searchResp.CurrentPage >= searchResp.TotalPages {
						break
					}
					searchOpt.Page = searchResp.NextPage
				}
			}

			mu.Lock()
			allResults = append(allResults, localResults...)
			mu.Unlock()
		}(pid)
	}

	wg.Wait()

	return allResults, nil
}
