package domainlist

import "testing"

func TestRuleType_String(t *testing.T) {
	if got := RuleFull.String(); got != "full" {
		t.Errorf("RuleFull.String() = %q, want %q", got, "full")
	}
	if got := RuleRootDomain.String(); got != "root" {
		t.Errorf("RuleRootDomain.String() = %q, want %q", got, "root")
	}
	if got := RuleRegex.String(); got != "regex" {
		t.Errorf("RuleRegex.String() = %q, want %q", got, "regex")
	}
	if got := RulePlain.String(); got != "plain" {
		t.Errorf("RulePlain.String() = %q, want %q", got, "plain")
	}
}

func TestRuleType_IsResolvable(t *testing.T) {
	if got := RuleFull.IsResolvable(); got != true {
		t.Errorf("RuleFull.IsResolvable() = %v, want %v", got, true)
	}
	if got := RuleRootDomain.IsResolvable(); got != true {
		t.Errorf("RuleRootDomain.IsResolvable() = %v, want %v", got, true)
	}
	if got := RuleRegex.IsResolvable(); got != false {
		t.Errorf("RuleRegex.IsResolvable() = %v, want %v", got, false)
	}
	if got := RulePlain.IsResolvable(); got != false {
		t.Errorf("RulePlain.IsResolvable() = %v, want %v", got, false)
	}
}
