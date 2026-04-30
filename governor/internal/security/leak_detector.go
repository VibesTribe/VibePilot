package security

import (
	"log"
	"regexp"
)

type LeakWarning struct {
	Pattern string
	Action  string
	Found   string
}

type leakPattern struct {
	name   string
	regex  *regexp.Regexp
	action string
}

type LeakDetector struct {
	patterns []leakPattern
}

func NewLeakDetector() *LeakDetector {
	return &LeakDetector{
		patterns: []leakPattern{
			{name: "openai_key", regex: regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`), action: "block"},
			{name: "anthropic_key", regex: regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]{80,}`), action: "block"},
			{name: "github_token", regex: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,}`), action: "block"},
			{name: "supabase_key", regex: regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`), action: "block"},
			{name: "aws_key", regex: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), action: "block"},
			{name: "generic_secret", regex: regexp.MustCompile(`(?i)(secret|password|token|key|api_key)\s*[=:]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`), action: "warn"},
		},
	}
}

func (d *LeakDetector) Scan(output string) (string, []LeakWarning) {
	var warnings []LeakWarning
	cleanOutput := output

	for _, p := range d.patterns {
		matches := p.regex.FindAllString(output, -1)
		if len(matches) > 0 {
			for _, match := range matches {
				warnings = append(warnings, LeakWarning{
					Pattern: p.name,
					Action:  p.action,
					Found:   maskSecret(match),
				})
			}

			if p.action == "block" {
				cleanOutput = p.regex.ReplaceAllString(cleanOutput, "[REDACTED:"+p.name+"]")
				log.Printf("LeakDetector: BLOCKED %s pattern found", p.name)
			} else if p.action == "warn" {
				log.Printf("LeakDetector: WARNING %s pattern found", p.name)
			}
		}
	}

	return cleanOutput, warnings
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
