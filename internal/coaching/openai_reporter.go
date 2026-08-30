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

type reportModelInput struct {
	MatchID int64                    `json:"match_id"`
	Hero    string                   `json:"hero"`
	Moments []reportModelInputMoment `json:"moments"`
}

type reportModelInputMoment struct {
	SourceMomentID string  `json:"source_moment_id"`
	Type           string  `json:"type"`
	StartT         float64 `json:"start_t"`
	EndT           float64 `json:"end_t"`
	Confidence     string  `json:"confidence"`
	Evidence       any     `json:"evidence"`
}

func buildReportModelInput(input MatchCoachingInput) reportModelInput {
	out := reportModelInput{
		MatchID: input.MatchID,
		Hero:    input.Hero,
		Moments: make([]reportModelInputMoment, 0, len(input.Moments)),
	}
	for index, moment := range input.Moments {
		out.Moments = append(out.Moments, reportModelInputMoment{
			SourceMomentID: sourceMomentID(index, moment),
			Type:           moment.Type,
			StartT:         moment.StartT,
			EndT:           moment.EndT,
			Confidence:     moment.Confidence,
			Evidence:       moment.Evidence,
		})
	}
	return out
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

	inputJSON, err := json.Marshal(buildReportModelInput(input))
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
			{"role": "user", "content": "MatchCoachingInput-derived evidence. Every moment has an explicit source_moment_id. Never refer to moments by array position; copy the exact source_moment_id of every moment whose evidence you use.\n" + string(inputJSON)},
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

You receive ONLY detector-normalized MatchCoachingInput-derived evidence. Treat it as the complete evidence available to you. Never invent or infer exact enemy locations, hidden vision, wards, player intent, map coordinates, buyback state, cooldowns, resources, or any other replay fact that is not explicitly present in the input.

Each input moment has a unique source_moment_id. Treat that ID as part of the evidence record. Never refer to a moment by its array position. For every fact or conclusion you use, copy the exact source_moment_id of the moment that supplied that evidence into source_moment_ids. Do not copy facts from a neighboring moment under another moment's ID.

Each input moment is a review target, not proof of a mistake. Preserve that uncertainty. The output assessment may be "review", "likely_mistake", or "probably_reasonable"; never describe a candidate as definitively wrong merely because a detector emitted it.

Select at most five high-value, actionable decisions. Prefer moments where a plausible alternative can be explained. Group overlapping source moments into one report moment when they concern the same underlying decision, and do not reuse a source moment in multiple report moments. When a specific mechanic-level interaction and a generic symptom overlap, prefer the more direct mechanic evidence as the root-cause review. Do not elevate a repeated low-confidence family merely because it appears more often; consider evidence directness, specificity, duration, and gameplay impact.

Strict temporal rule:
- decision_time_facts may contain only facts available at or before the reviewed decision;
- retrospective_outcomes are later replay results used only to show consequence or prioritize the review;
- evidence such as next_teamfight_start_t, seconds_until_teamfight, target_dead_at_teamfight_start, next fight statistics, next_target_death_t, later deaths, later conversions, later fight outcomes, damage after a key-ability cast, deaths after a cast, reflected damage after a cast, and target_death_t is retrospective unless the input explicitly says it was known at decision time;
- never use retrospective outcomes to claim the player should have predicted an upcoming, approaching, imminent, or next fight;
- never use retrospective outcomes to justify the alternative action or to make the assessment stronger;
- interpretation and alternative must be supported by decision-time evidence only;
- why_it_matters may mention retrospective outcomes, clearly as hindsight consequence.

Key-ability review moments are deliberately emitted for supported casts whether the later result is good, bad, or ambiguous. A later death does not make the cast bad, and a later enemy death does not automatically make it good. Use pre-cast state for the decision assessment and later damage/deaths only as retrospective outcome. It is valid to assess a supported cast as probably_reasonable when the supplied decision-time evidence supports that conclusion.

If you select a key-ability review or mechanic-level interaction and the input contains other casts of that same key ability, reserve one report slot for one contrasting cast when the evidence supports a useful comparison. The comparison exists to calibrate the coaching, not to reward or punish the later outcome. Choose a cast with meaningfully different supplied context or consequence, keep later damage/deaths retrospective, and assess it as review or probably_reasonable only when its decision-time evidence supports that wording. Do not drop this calibration cast merely to add another generic repeated low-confidence symptom.

For active-damage-reflect interaction evidence, item_use_t is replay-recorded truth before the reviewed cast. If player_knowledge_status is "not_confirmed_from_replay", never say the player definitely saw, knew, or should have known the item activation. You may cautiously state that the replay records the activation before the cast and frame the coaching question as whether the cue was noticed or respected. reflected_damage_after_cast, first_reflected_damage_t, target_death_to_reflect, and target_death_t are retrospective outcomes only.

Isolation evidence contains an internal timeline-coordinate distance. Do not quote support_radius_timeline or nearest_ally_distance in prose. Use nearby_allies_within_support together with support_radius_world instead. For example, if nearby_allies_within_support is 0 and support_radius_world is 1500, say no ally was within the configured 1500-world-unit support radius. Do not convert or expose internal coordinate units.

Missing-enemy evidence is conservative last-seen knowledge, not a known enemy position. You may state that an enemy had not been seen for the supplied duration/status, but never infer where that enemy was.

For every selected report moment:
- source_moment_ids must contain the exact IDs of the input moments that support it;
- decision_time_facts and retrospective_outcomes must be direct, cautious restatements of explicit evidence from those exact source IDs;
- interpretation must clearly be coaching inference rather than a new replay fact;
- alternative must describe a plausible action available around that decision time without relying on later events;
- why_it_matters should explain the practical gameplay consequence.

Summary and priorities must also avoid hindsight-as-knowledge. They may identify repeated behavior and retrospective cost, but priorities must be actionable from information available during play.

Do not narrate the replay chronologically. Do not add generic Dota advice unrelated to the selected evidence. If the evidence does not support a useful conclusion, omit that moment. Keep the summary and priorities concise and specific to the selected decisions.`