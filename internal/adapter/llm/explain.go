package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"linewatch/internal/domain"
)

const model = "openrouter/free"

func proseLanguage(lang string) string {
	if lang == "en" {
		return "English"
	}
	return "Spanish"
}

type Explainer struct {
	key    string
	client *http.Client
}

func New(key string) Explainer {
	return Explainer{key: key, client: &http.Client{Timeout: 20 * time.Second}}
}

func (e Explainer) Explain(ev domain.Evidence) (string, string) {
	fallback, action := domain.Template(ev)
	if e.key == "" {
		return fallback, action
	}
	reason, rec, ok := e.complete(ev)
	if !ok || !numbersFit(reason+" "+rec, ev) {
		return fallback, action
	}
	return reason, rec
}

func (e Explainer) complete(ev domain.Evidence) (string, string, bool) {
	packet, err := json.Marshal(ev)
	if err != nil {
		return "", "", false
	}
	body, err := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "Write reason and recommended_action in " + proseLanguage(ev.Language) + ". Use only numbers from the data block. Return JSON with those two fields."},
			{"role": "user", "content": "DATA:\n" + string(packet)},
		},
		"max_tokens": 300,
	})
	if err != nil {
		return "", "", false
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", false
	}
	req.Header.Set("Authorization", "Bearer "+e.key)
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		return "", "", false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", "", false
	}
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil || len(payload.Choices) == 0 {
		return "", "", false
	}
	text := payload.Choices[0].Message.Content
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var out struct {
		Reason string `json:"reason"`
		Action string `json:"recommended_action"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &out); err != nil {
		return "", "", false
	}
	if out.Reason == "" || out.Action == "" {
		return "", "", false
	}
	return out.Reason, out.Action, true
}

var numberRe = regexp.MustCompile(`\d+(?:\.\d+)?`)

func numbersFit(text string, ev domain.Evidence) bool {
	allowed := []float64{
		ev.BaselineKWh, ev.ActualKWh, ev.VariationPct, ev.Confidence,
		ev.CurrentA, ev.VoltageV, ev.PowerFactor, ev.Confidence * 100,
	}
	for _, raw := range numberRe.FindAllString(text, -1) {
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return false
		}
		if !closeTo(n, allowed) {
			return false
		}
	}
	return true
}

func closeTo(n float64, allowed []float64) bool {
	for _, a := range allowed {
		if abs(n-a) <= 0.15 || abs(n-float64(int(a))) <= 0.15 {
			return true
		}
	}
	return false
}

func abs(n float64) float64 {
	if n < 0 {
		return -n
	}
	return n
}
