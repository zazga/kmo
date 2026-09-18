package ui

import "testing"

func TestFuzzyMatch(t *testing.T) {
	cases := []struct { candidate, query string; want bool }{{"development","dev",true},{"general-chat","gch",true},{"alice smith alice","asm",true},{"random","zzz",false}}
	for _, tc := range cases { if got := FuzzyMatch(tc.candidate, tc.query); got != tc.want { t.Fatalf("FuzzyMatch(%q,%q)=%v want %v", tc.candidate, tc.query, got, tc.want) } }
}
