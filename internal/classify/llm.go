package classify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type LLMSuggestion struct {
	Category string `json:"category"`
	Reason   string `json:"reason"`
	Confidence string `json:"confidence"` // low|med|high
}

// SuggestCategoryLLM calls `codex exec` to propose a category.
// This is optional and should be used as assist-only.
func SuggestCategoryLLM(ctx context.Context, merchantRaw, details string, amountCents int64, existingCategories []string) (*LLMSuggestion, error) {
	prompt := fmt.Sprintf(`You are helping classify personal finance transactions.

Merchant: %s
Details: %s
Amount (cents, negative means spend): %d

Known categories (pick one if appropriate):
%s

Return JSON only:
{"category":"...","reason":"...","confidence":"low|med|high"}
`, strings.TrimSpace(merchantRaw), strings.TrimSpace(details), amountCents, strings.Join(existingCategories, ", "))

	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, "codex", "exec", prompt)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex exec failed: %w: %s", err, strings.TrimSpace(out.String()))
	}

	b := bytes.TrimSpace(out.Bytes())
	var sug LLMSuggestion
	if err := json.Unmarshal(b, &sug); err != nil {
		// try to salvage by extracting last {...}
		s := out.String()
		i := strings.LastIndex(s, "{")
		j := strings.LastIndex(s, "}")
		if i >= 0 && j > i {
			if err2 := json.Unmarshal([]byte(s[i:j+1]), &sug); err2 == nil {
				return &sug, nil
			}
		}
		return nil, fmt.Errorf("codex output was not JSON: %s", strings.TrimSpace(out.String()))
	}
	return &sug, nil
}
