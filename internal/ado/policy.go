package ado

import (
	"context"
	"fmt"
	"net/url"
)

const policyAPIVersion = "7.1-preview.1"

// PolicyEvaluation is the state of one branch policy on one pull request.
type PolicyEvaluation struct {
	Status        *string `json:"status"`
	StartedDate   *string `json:"startedDate"`
	CompletedDate *string `json:"completedDate"`
	Configuration *struct {
		ID         int            `json:"id"`
		IsBlocking bool           `json:"isBlocking"`
		IsEnabled  bool           `json:"isEnabled"`
		Settings   map[string]any `json:"settings"`
		Type       *struct {
			DisplayName *string `json:"displayName"`
		} `json:"type"`
	} `json:"configuration"`
}

// PolicyEvaluations lists the policy evaluations of a pull request in the project with ID
// projectID.
func (c *Client) PolicyEvaluations(ctx context.Context, orgURL, projectID string, pullRequestID int) ([]PolicyEvaluation, error) {
	query := url.Values{
		"artifactId":  {fmt.Sprintf("vstfs:///CodeReview/CodeReviewId/%s/%d", projectID, pullRequestID)},
		"api-version": {policyAPIVersion},
	}
	return getValues[PolicyEvaluation](ctx, c, projectURL(orgURL, projectID)+"/_apis/policy/evaluations?"+encodeQuery(query))
}
