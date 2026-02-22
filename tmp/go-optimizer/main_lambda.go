//go:build lambda

package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

//go:embed data.min.json
var embeddedData string

var jsonHeader = map[string]string{
	"Content-Type": "application/json",
}

type optimizeRequest struct {
	RuleID           int             `json:"ruleId"`
	Archive          json.RawMessage `json:"archive"`
	ExcludeMaterials []int           `json:"excludeMaterials"`
}

type optimizeResult struct {
	RuleID int    `json:"ruleId"`
	Name   string `json:"name"`
	Score  int    `json:"score"`
	TimeMs int64  `json:"timeMs"`
	Detail string `json:"detail"`
}

func handler(_ context.Context, event events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	body := event.Body
	if event.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return errResp(400, "invalid base64 body")
		}
		body = string(decoded)
	}

	var req optimizeRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return errResp(400, "invalid JSON: "+err.Error())
	}
	if len(req.Archive) == 0 {
		return errResp(400, "missing archive field")
	}
	if req.RuleID == 0 {
		return errResp(400, "missing ruleId")
	}

	input := loadFromStrings(embeddedData, string(req.Archive))
	gd := input.ToGameData()

	contest := FindContest(input, req.RuleID)
	if contest == nil {
		return errResp(404, fmt.Sprintf("ruleId %d not found", req.RuleID))
	}

	if len(req.ExcludeMaterials) > 0 {
		excludeSet := make(map[int]bool, len(req.ExcludeMaterials))
		for _, mid := range req.ExcludeMaterials {
			excludeSet[mid] = true
		}
		for ri := range contest.Rules {
			filtered := contest.Rules[ri].Recipes[:0]
			for _, r := range contest.Rules[ri].Recipes {
				excluded := false
				for _, m := range r.Materials {
					if excludeSet[m.Material] {
						excluded = true
						break
					}
				}
				if !excluded {
					filtered = append(filtered, r)
				}
			}
			contest.Rules[ri].Recipes = filtered
		}
	}

	r, bestState := runContest(contest, gd, DefaultConfig())
	detail := ""
	if bestState != nil {
		detail = FormatResult(contest.Rules, bestState, gd, contest)
	}

	resp := optimizeResult{
		RuleID: r.RuleID, Name: r.Name, Score: r.Score, TimeMs: r.TimeMs, Detail: detail,
	}
	respJSON, _ := json.Marshal(resp)
	return events.LambdaFunctionURLResponse{StatusCode: 200, Headers: jsonHeader, Body: string(respJSON)}, nil
}

func errResp(code int, msg string) (events.LambdaFunctionURLResponse, error) {
	body, _ := json.Marshal(map[string]string{"error": msg})
	return events.LambdaFunctionURLResponse{StatusCode: code, Headers: jsonHeader, Body: string(body)}, nil
}

func main() {
	lambda.Start(handler)
}
