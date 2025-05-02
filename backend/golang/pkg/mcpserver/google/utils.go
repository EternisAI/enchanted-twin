package google

import (
	"context"
	"fmt"
	"time"

	"github.com/EternisAI/enchanted-twin/pkg/db"
)

type TimeRange struct {
	From uint64 `json:"from" jsonschema:",description=The start timestamp in seconds of the time range, default is 0"`
	To   uint64 `json:"to"   jsonschema:",description=The end timestamp in seconds of the time range, default is 0"`
}

// A relative time span such as "last 2 weeks" or "next 24 hours".
type RelativeSpan struct {
	// Allowed units; use string not enum to keep the JSON tiny & obvious.
	// Valid values: "hours", "days", "weeks", "months", "years".
	Unit string `json:"unit" jsonschema:"enum=hours,days,weeks,months,years"`

	// The positive integer count of those units.
	// Example: 2 → "last 2 weeks"
	Value uint `json:"value" jsonschema:"minimum=1"`

	// Direction of the span relative to *now*.
	// "past"  → look backwards  (last / past / before now)
	// "future"→ look forwards   (next / upcoming / until)
	Direction string `json:"direction" jsonschema:"enum=past,future"`
}

// The user can express EITHER a relative span OR an absolute ‘before’ / ‘after’.
type TimeFilter struct {
	// ONE of the following must be non-zero / non-empty

	// Option A: relative span such as {value:3, unit:"months", direction:"past"}.
	Span *RelativeSpan `json:"span,omitempty"`

	// Option B: absolute cutoff – everything BEFORE this Unix second.
	Before string `json:"before,omitempty" jsonschema:"description=The time to filter emails before, in RFC3339 format (e.g. 2024-01-01T00:00:00Z)"`

	// Option C: absolute cutoff – everything AFTER this Unix second.
	After string `json:"after,omitempty" jsonschema:"description=The time to filter emails after, in RFC3339 format (e.g. 2024-01-01T00:00:00Z)"`
}

func (tf TimeFilter) ToUnixRange(now time.Time) (start, end uint64, err error) {
	switch {
	//-----------------------------------------------------------------
	case tf.Span != nil: // relative span, e.g., last 2 months
		d, derr := spanToDuration(*tf.Span)
		if derr != nil {
			return 0, 0, derr
		}

		if tf.Span.Direction == "past" {
			start = uint64(now.Add(-d).Unix())
			end = uint64(now.Unix())
		} else { // future
			start = uint64(now.Unix())
			end = uint64(now.Add(d).Unix())
		}

	//-----------------------------------------------------------------
	case tf.Before != "" && tf.After == "": // open start, closed end
		start = 0
		endTime, err := time.Parse(time.RFC3339, tf.Before)
		if err != nil {
			return 0, 0, err
		}
		end = uint64(endTime.Unix())

	//-----------------------------------------------------------------
	case tf.After != "" && tf.Before == "": // closed start, open end
		startTime, err := time.Parse(time.RFC3339, tf.After)
		if err != nil {
			return 0, 0, err
		}
		start = uint64(startTime.Unix())
		end = 0

	//-----------------------------------------------------------------
	case tf.After != "" && tf.Before != "": // closed start, closed end
		startTime, err := time.Parse(time.RFC3339, tf.After)
		if err != nil {
			return 0, 0, err
		}
		start = uint64(startTime.Unix())
		endTime, err := time.Parse(time.RFC3339, tf.Before)
		if err != nil {
			return 0, 0, err
		}
		end = uint64(endTime.Unix())
	//-----------------------------------------------------------------
	default:
		start = 0
		end = 0
	}
	return start, end, err
}

func spanToDuration(s RelativeSpan) (time.Duration, error) {
	if s.Value == 0 {
		return 0, fmt.Errorf("span value must be >0")
	}

	switch s.Unit {
	case "hours":
		return time.Duration(s.Value) * time.Hour, nil
	case "days":
		return time.Duration(s.Value) * 24 * time.Hour, nil
	case "weeks":
		return time.Duration(s.Value) * 7 * 24 * time.Hour, nil
	case "months":
		// Months/years are variable; use AddDate for accurate math
		return time.Duration(s.Value) * 30 * 24 * time.Hour, nil // coarse fallback
	case "years":
		return time.Duration(s.Value) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid unit %q", s.Unit)
	}
}

func GetAccessToken(ctx context.Context, store *db.Store, emailAccount string) (string, error) {
	oauthTokens, err := store.GetOAuthTokensArray(ctx, "google")
	if err != nil {
		return "", err
	}
	var accessToken string
	for _, oauthToken := range oauthTokens {
		if oauthToken.Username == emailAccount {
			accessToken = oauthToken.AccessToken
			break
		}
	}
	if accessToken == "" {
		return "", fmt.Errorf("email account not found")
	}
	return accessToken, nil
}
