package linear

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message"`
	Path    []string `json:"path"`
}

type Issue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       IssueState `json:"state"`
	CompletedAt *time.Time `json:"completedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Team        Team      `json:"team"`
	Assignee    *User     `json:"assignee"`
}

type IssueState struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Team struct {
	Name string `json:"name"`
}

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func NewClient(apiKey string) *Client {
	if apiKey == "" {
		log.Fatal("LINEAR_API_KEY is required")
	}

	log.Println("linear client initialized")

	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.linear.app/graphql",
	}
}

func (c *Client) query(query string, variables map[string]interface{}) (json.RawMessage, error) {
	reqBody := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Linear API error (status %d): %s", resp.StatusCode, string(body))
	}

	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data, nil
}

func (c *Client) GetRecentlyCompletedIssues(days int) ([]Issue, error) {
	log.Printf("fetching completed issues from the last %d days", days)

	threshold := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	query := `
		query($filter: IssueFilter) {
			issues(filter: $filter, first: 50) {
				nodes {
					id
					title
					description
					state {
						name
						type
					}
					completedAt
					updatedAt
					team {
						name
					}
					assignee {
						name
						email
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"filter": map[string]interface{}{
			"completedAt": map[string]interface{}{
				"gte": threshold,
			},
		},
	}

	data, err := c.query(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Issues struct {
			Nodes []Issue `json:"nodes"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse issues: %w", err)
	}

	log.Printf("found %d completed issues", len(result.Issues.Nodes))

	return result.Issues.Nodes, nil
}

func (c *Client) GetIssue(issueID string) (*Issue, error) {
	query := `
		query($id: String!) {
			issue(id: $id) {
				id
				title
				description
				state {
					name
					type
				}
				completedAt
				updatedAt
				team {
					name
				}
				assignee {
					name
					email
				}
			}
		}
	`

	variables := map[string]interface{}{
		"id": issueID,
	}

	data, err := c.query(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Issue Issue `json:"issue"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse issue: %w", err)
	}

	return &result.Issue, nil
}

func (c *Client) GetMyIssues(filter string) ([]Issue, error) {
	query := `
		query {
			viewer {
				assignedIssues(first: 50) {
					nodes {
						id
						title
						description
						state {
							name
							type
						}
						completedAt
						updatedAt
						team {
							name
						}
					}
				}
			}
		}
	`

	data, err := c.query(query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Viewer struct {
			AssignedIssues struct {
				Nodes []Issue `json:"nodes"`
			} `json:"assignedIssues"`
		} `json:"viewer"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse issues: %w", err)
	}

	return result.Viewer.AssignedIssues.Nodes, nil
}