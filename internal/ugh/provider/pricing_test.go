package provider

import (
	"strings"
	"testing"
)

func TestParseModelsPricingAndLookup(t *testing.T) {
	ms, err := parseModels("openrouter", strings.NewReader(`{"data":[
		{"id":"a/cheap","context_length":1000,"pricing":{"prompt":"0.000001","completion":"0.000004"}},
		{"id":"auto","pricing":{"prompt":"-1","completion":"-1"}},
		{"id":"free","pricing":{"prompt":"0","completion":"0"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cacheData = nil
	storeModels("openrouter", ms)

	in, out, ok := Pricing("openrouter/a/cheap")
	if !ok || in != 1e-6 || out != 4e-6 {
		t.Fatalf("cheap = %v %v %v", in, out, ok)
	}
	for _, ref := range []string{"openrouter/auto", "openrouter/free", "openrouter/missing", "bare"} {
		if _, _, ok := Pricing(ref); ok {
			t.Errorf("%s should be unpriced", ref)
		}
	}
}
