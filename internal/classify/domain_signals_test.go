// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
 "os"
 "regexp"
 "testing"
)

func TestDomainSignalsIntegrity(t *testing.T) {
 seen := make(map[string]bool)
 domains := make(map[string]bool)
 for _, row := range RawDomainSignals {
   if row.ID == "" || row.Domain == "" || seen[row.ID] { t.Fatalf("invalid or duplicate signal: %+v", row) }
   seen[row.ID] = true
   domains[row.Domain] = true
   if row.Weight < 1 || row.Weight > 3 { t.Fatalf("invalid weight: %s", row.ID) }
   if row.Class != Medium && row.Class != Complex { t.Fatalf("unreviewed complexity: %s", row.ID) }
   if _, err := regexp.Compile("(?i)" + row.Pattern); err != nil { t.Fatalf("invalid regex %s: %v", row.ID, err) }
 }
 if len(domains) != 8 { t.Fatalf("got %d domains, expected 8", len(domains)) }
 if len(RawDomainSignals) != 24 { t.Fatalf("got %d rules, expected 24", len(RawDomainSignals)) }
}

func TestDomainSignalsOptIn(t *testing.T) {
 previous, existed := os.LookupEnv("DOWNSHIFT_DOMAIN_SIGNALS")
 t.Cleanup(func() { if existed { os.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", previous) } else { os.Unsetenv("DOWNSHIFT_DOMAIN_SIGNALS") } })
 os.Unsetenv("DOWNSHIFT_DOMAIN_SIGNALS")
 if len(activeSignals()) != len(signals) { t.Fatal("default must use baseline") }
 os.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "1")
 if len(activeSignals()) != len(signals)+len(RawDomainSignals) { t.Fatal("opt-in must append domain rules") }
 os.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "0")
 if len(activeSignals()) != len(signals) { t.Fatal("opt-out must restore baseline") }
}

func TestDomainSignalsNoAutomaticFrameworkEscalation(t *testing.T) {
 previous, existed := os.LookupEnv("DOWNSHIFT_DOMAIN_SIGNALS")
 t.Cleanup(func() { if existed { os.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", previous) } else { os.Unsetenv("DOWNSHIFT_DOMAIN_SIGNALS") } })
 os.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "1")
 for _, word := range []string{"React", "Terraform", "Kubernetes", "Flutter", "Helm"} {
  got := Classify(word)
  if got.Complexity == Complex && got.Confident { t.Errorf("%q escalated to frontier on framework name alone", word) }
 }
}
