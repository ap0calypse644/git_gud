package coaching

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultOpenAIBaseURL         = "https://api.openai.com/v1"
	defaultOpenAIModel           = "gpt-5.6-terra"
	defaultReportMaxOutputTokens = 3000
)

// ReportGenerator is the Phase G boundary. Implementations receive only the
// compact MatchCoachingInput; raw MatchTimeline values must never cross here.
type ReportGenerator interface {
	Generate(ctx context.Context, input MatchCoachingInput) (MatchCoachingReport, error)
}

// OpenAIReporter generates one structured coaching report through the Responses
// API. It intentionally uses the standard library so Phase G adds no SDK
// dependency and the remote provider remains isolated behind ReportGenerator.
type OpenAIReporter struct {
	APIKey          string
	BaseURL         string
	Model           string
	MaxOutputTokens int
	Client          *http.Client
}

func NewOpenAIReporter(apiKey, model string, client *http.Client) *OpenAIReporter {
	if strings.TrimSpace(model) == "" {
		model = defaultOpenAIModel
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &OpenAIReporter{
		APIKey:          strings.TrimSpace(apiKey),
		BaseURL:         defaultOpenAIBaseURL,
		Model:           strings.TrimSpace(model),
		MaxOutputTokens: defaultReportMaxOutputTokens,
		Client:          client,
	}
}

func (r *OpenAIReporter) Generate(ctx context.Context, input MatchCoachingInput) (MatchCoachingReport, error) {
	if r == nil {
		return MatchCoachingReport{}, fmt.Errorf("openai reporter is nil")
	}
	if strings.TrimSpace(r.APIKey) == "" {
		return MatchCoachingReport{}, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if r.Client == nil {
		return MatchCoachingReport{}, fmt.Errorf("openai http client is nil")
	}
	if strings.TrimSpace(r.Model) == "" {
		return MatchCoachingReport{}, fmt.Errorf("openai model is required")
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return MatchCoachingReport{}, fmt.Errorf("encode coaching input: %w", err)
	}

	maxOutputTokens := r.MaxOutputTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultReportMaxOutputTokens
	}
	requestBody := map[string]any{
		"model":             r.Model,
		"store":             false,
		"max_output_tokens": maxOutputTokens,
		"input": []map[string]string{
			{"role": "system", "content": coachingReportSystemPrompt},
			{"role": "user", "content": "MatchCoachingInput. Source moment indexes are zero-based indexes into moments.\n" + string(inputJSON)},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "git_gud_match_coaching_report",
				"strict": true,
				"schema": coachingReportJSONSchema(),
			},
		},
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return MatchCoachingReport{}, fmt.Errorf("encode OpenAI request: %w", err)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(r.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/responses", bytes.NewReader(encoded))
	if err != nil {
		return MatchCoachingReport{}, fmt.Errorf("create OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.Client.Do(req)
	if err != nil {
		return MatchCoachingReport{}, fmt.Errorf("OpenAI responses request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return MatchCoachingReport{}, fmt.Errorf("read OpenAI response: %w", err)
	}

	var apiResponse openAIResponsesAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return MatchCoachingReport{}, fmt.Errorf("decode OpenAI response (status %s): %w", resp.Status, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(apiResponse.Error.Message)
		if message == "" {
			message = strings.TrimSpace(string(body))
		}
		return MatchCoachingReport{}, fmt.Errorf("OpenAI responses API %s: %s", resp.Status, message)
	}
	if apiResponse.Status == "incomplete" {
		reason := strings.TrimSpace(apiResponse.IncompleteDetails.Reason)
		if reason == "" {
			reason = "unknown reason"
		}
		return MatchCoachingReport{}, fmt.Errorf("OpenAI response incomplete: %s", reason)
	}

	outputText, err := extractOpenAIOutputText(apiResponse)
	if err != nil {
		return MatchCoachingReport{}, err
	}
	var modelOutput modelReportOutput
	if err := json.Unmarshal([]byte(outputText), &modelOutput); err != nil {
		return MatchCoachingReport{}, fmt.Errorf("decode structured coaching report: %w", err)
	}
	return buildMatchCoachingReport(input, modelOutput)
}

type openAIResponsesAPIResponse struct {
	Status            string `json:"status"`
	IncompleteDetails struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		} `json:"content"`
	} `json:"output"`
}

func extractOpenAIOutputText(response openAIResponsesAPIResponse) (string, error) {
	for _, item := range response.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			switch content.Type {
			case "refusal":
				if strings.TrimSpace(content.Refusal) == "" {
					return "", fmt.Errorf("OpenAI refused the coaching request")
				}
				return "", fmt.Errorf("OpenAI refused the coaching request: %s", content.Refusal)
			case "output_text":
				if strings.TrimSpace(content.Text) != "" {
					return content.Text, nil
				}
			}
		}
	}
	return "", fmt.Errorf("OpenAI response contained no output_text")
}

const coachingReportSystemPrompt = `You are a Dota 2 replay coaching reviewer.

You receive ONLY detector-normalized MatchCoachingInput. Treat it as the complete evidence available to you. Never invent or infer exact enemy locations, hidden vision, wards, player intent, map coordinates, buyback state, cooldowns, resources, or any other replay fact that is not explicitly present in the input.

Each input moment is a review target, not proof of a mistake. Preserve that uncertainty. The output assessment may be "review", "likely_mistake", or "probably_reasonable"; never describe a candidate as definitively wrong merely because a detector emitted it.

Select at most five high-value, actionable decisions. Prefer moments where a plausible alternative can be explained. Group overlapping source moments into one report moment when they concern the same underlying decision, and do not reuse a source moment in multiple report moments.

Strict temporal rule:
- decision_time_facts may contain only facts available at or before the reviewed decision;
- retrospective_outcomes are later replay results used only to show consequence or prioritize the review;
- evidence such as next_teamfight_start_t, seconds_until_teamfight, target_dead_at_teamfight_start, next fight statistics, next_target_death_t, later deaths, later conversions, and later fight outcomes is retrospective unless the input explicitly says it was known at decision time;
- never use retrospective outcomes to claim the player should have predicted an upcoming, approaching, imminent, or next fight;
- never use retrospective outcomes to justify the alternative action or to make the assessment stronger;
- interpretation and alternative must be supported by decision-time evidence only;
- why_it_matters may mention retrospective outcomes, clearly as hindsight consequence.

Isolation evidence contains an internal timeline-coordinate distance. Do not quote support_radius_timeline or nearest_ally_distance in prose. Use nearby_allies_within_support together with support_radius_world instead. For example, if nearby_allies_within_support is 0 and support_radius_world is 1500, say no ally was within the configured 1500-world-unit support radius. Do not convert or expose internal coordinate units.

Missing-enemy evidence is conservative last-seen knowledge, not a known enemy position. You may state that an enemy had not been seen for the supplied duration/status, but never infer where that enemy was.

For every selected report moment:
- source_moment_indexes must reference the zero-based input moments that support it;
- decision_time_facts and retrospective_outcomes must be direct, cautious restatements of explicit source evidence;
- interpretation must clearly be coaching inference rather than a new replay fact;
- alternative must describe a plausible action available around that decision time without relying on later events;
- why_it_matters should explain the practical gameplay consequence.

Summary and priorities must also avoid hindsight-as-knowledge. They may identify repeated behavior and retrospective cost, but priorities must be actionable from information available during play.

Do not narrate the replay chronologically. Do not add generic Dota advice unrelated to the selected evidence. If the evidence does not support a useful conclusion, omit that moment. Keep the summary and priorities concise and specific to the selected decisions.`
