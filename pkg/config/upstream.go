package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type UpstreamConfig struct {
	URL     string            `yaml:"url"`
	Timeout time.Duration     `yaml:"timeout"`
	Headers map[string]string `yaml:"headers"`

	// FailOn defines which upstream HTTP status codes should be returned immediately
	// to the client without falling back to the generator.
	// nil (omitted): uses default (400-499 except 401, 403). Set to empty list (fail-on: []) to disable.
	FailOn *HTTPStatusMatchConfig `yaml:"fail-on"`

	// StickyTimeout enables server-side session affinity for upstream/generator routing.
	// When a client gets a generated (fallback) response, subsequent requests from the
	// same remote address skip upstream for this duration. 0 or omitted = disabled.
	StickyTimeout time.Duration `yaml:"sticky-timeout"`
}

// DefaultFailOnStatus is the default fail-on config applied when FailOn is nil.
// Most 4xx errors indicate client-side problems that the generator cannot fix.
// 401/403 are excluded because they typically indicate missing/invalid credentials
// in the proxy setup, not a real client error.
var DefaultFailOnStatus = HTTPStatusMatchConfig{
	{Range: "400-499", Except: []int{401, 403}},
}

// DefaultUpstreamTimeout defaults.
const (
	DefaultUpstreamTimeout = 5 * time.Second
)

type BodyMatchCondition struct {
	Path   string      `yaml:"path"`
	Equals interface{} `yaml:"equals"`
}

type BodyMatchConfig struct {
	Default string               `yaml:"default"`
	Fail    []BodyMatchCondition `yaml:"fail"`
	Except  []BodyMatchCondition `yaml:"except"`
}

func (b *BodyMatchConfig) defaultAction() string {
	if b.Default == "except" {
		return "except"
	}
	return "fail"
}

type HTTPStatusConfig struct {
	Exact  int              `yaml:"exact"`
	Range  string           `yaml:"range"`
	Except []int            `yaml:"except"`
	Body   *BodyMatchConfig `yaml:"body"`
}

func (s *HTTPStatusConfig) Is(status int, body string) bool {
	for _, ex := range s.Except {
		if ex == status {
			return false
		}
	}

	statusMatch := s.Exact == status

	if !statusMatch {
		rangeParts := strings.Split(s.Range, "-")
		if len(rangeParts) == 2 {
			lower, err1 := strconv.Atoi(rangeParts[0])
			upper, err2 := strconv.Atoi(rangeParts[1])
			if err1 == nil && err2 == nil && status >= lower && status <= upper {
				statusMatch = true
			}
		}
	}

	if !statusMatch {
		return false
	}

	if s.Body == nil {
		return true
	}

	failMatch := len(s.Body.Fail) > 0 && matchesBodyConditions(body, s.Body.Fail)
	exceptMatch := len(s.Body.Except) > 0 && matchesBodyConditions(body, s.Body.Except)

	if failMatch && exceptMatch {
		return s.Body.defaultAction() == "fail"
	}
	if failMatch {
		return true
	}
	if exceptMatch {
		return false
	}

	// Neither matched: fall back to default.
	return s.Body.defaultAction() == "fail"
}

// matchesBodyConditions parses body as JSON and checks all conditions match.
func matchesBodyConditions(body string, conditions []BodyMatchCondition) bool {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return false
	}

	for _, cond := range conditions {
		parts := strings.Split(cond.Path, ".")
		var current interface{} = data
		for _, part := range parts {
			m, ok := current.(map[string]interface{})
			if !ok {
				return false
			}
			current, ok = m[part]
			if !ok {
				return false
			}
		}

		if fmt.Sprintf("%v", current) != fmt.Sprintf("%v", cond.Equals) {
			return false
		}
	}

	return true
}

type HTTPStatusMatchConfig []HTTPStatusConfig

func (ss HTTPStatusMatchConfig) Is(status int, body string) bool {
	for _, s := range ss {
		if s.Is(status, body) {
			return true
		}
	}

	return false
}
