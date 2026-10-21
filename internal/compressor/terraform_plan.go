package compressor

import (
	"bytes"
	"regexp"
	"unicode/utf8"
)

// Terraform plan output patterns
var (
	reTerraformPlan      = regexp.MustCompile(`(?m)^Terraform (will perform|used the selected)`)
	reTerraformNoChanges = regexp.MustCompile(`(?m)^No changes\. (Your infrastructure matches|Infrastructure is up-to-date)`)
	reTerraformResource  = regexp.MustCompile(`(?m)^  # (\S+) (will be|must be|has been) (created|destroyed|updated|replaced|read)`)
	reTerraformPlanLine  = regexp.MustCompile(`(?m)^Plan: (\d+) to add, (\d+) to change, (\d+) to destroy`)
	reTerraformSensitive = regexp.MustCompile(`(?i)(password|secret|key|token|credential)`)
)

func isTerraformPlan(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return reTerraformPlan.Match(input) || reTerraformNoChanges.Match(input)
}

func compressTerraformPlan(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "terraform_too_large"
	}

	// No changes case — super compact
	if reTerraformNoChanges.Match(input) {
		return []byte("terraform plan: no changes"), true, "terraform_no_changes"
	}

	lines := bytes.Split(input, []byte("\n"))
	var out bytes.Buffer

	// Extract summary line first
	var planSummary []byte
	for _, line := range lines {
		if reTerraformPlanLine.Match(line) {
			planSummary = line
			break
		}
	}

	// Count resources by action
	var toCreate, toDestroy, toUpdate, toReplace, toRead int
	resourceNames := make(map[string][]string) // action -> resource names

	for _, line := range lines {
		if matches := reTerraformResource.FindSubmatch(line); matches != nil {
			resource := string(matches[1])
			action := string(matches[3])

			switch action {
			case "created":
				toCreate++
				resourceNames["create"] = append(resourceNames["create"], resource)
			case "destroyed":
				toDestroy++
				resourceNames["destroy"] = append(resourceNames["destroy"], resource)
			case "updated":
				toUpdate++
				resourceNames["update"] = append(resourceNames["update"], resource)
			case "replaced":
				toReplace++
				resourceNames["replace"] = append(resourceNames["replace"], resource)
			case "read":
				toRead++
			}
		}
	}

	// Build compact output
	out.WriteString("terraform plan:\n")

	if planSummary != nil {
		out.Write(bytes.TrimSpace(planSummary))
		out.WriteString("\n\n")
	}

	// List resources by action (most important first: destroy, replace, create, update)
	if toDestroy > 0 {
		out.WriteString("DESTROY:\n")
		for _, r := range resourceNames["destroy"] {
			out.WriteString("  - ")
			out.WriteString(r)
			out.WriteString("\n")
		}
	}

	if toReplace > 0 {
		out.WriteString("REPLACE:\n")
		for _, r := range resourceNames["replace"] {
			out.WriteString("  - ")
			out.WriteString(r)
			out.WriteString("\n")
		}
	}

	if toCreate > 0 {
		out.WriteString("CREATE:\n")
		for i, r := range resourceNames["create"] {
			if i >= 10 {
				out.WriteString("  ... and ")
				out.WriteString(itoa(len(resourceNames["create"]) - 10))
				out.WriteString(" more\n")
				break
			}
			out.WriteString("  + ")
			out.WriteString(r)
			out.WriteString("\n")
		}
	}

	if toUpdate > 0 {
		out.WriteString("UPDATE:\n")
		for i, r := range resourceNames["update"] {
			if i >= 10 {
				out.WriteString("  ... and ")
				out.WriteString(itoa(len(resourceNames["update"]) - 10))
				out.WriteString(" more\n")
				break
			}
			out.WriteString("  ~ ")
			out.WriteString(r)
			out.WriteString("\n")
		}
	}

	if toRead > 0 {
		out.WriteString("READ: ")
		out.WriteString(itoa(toRead))
		out.WriteString(" data sources\n")
	}

	// Warn about sensitive attributes
	if reTerraformSensitive.Match(input) {
		out.WriteString("\n⚠️  Plan contains sensitive attributes\n")
	}

	result := out.Bytes()
	if len(result) >= len(input) {
		return input, false, "terraform_no_gain"
	}

	return result, true, "terraform_compacted"
}
