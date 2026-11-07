// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedact_AWSSTSJSON(t *testing.T) {
	asiaKey := "ASIA" + "IOSFODNN7EXAMPLE"
	secretKey := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	sessionToken := "AQoDYXdzEJr" + "1111111111111111EXAMPLETOKEN"

	tests := []struct {
		name string
		in   string
	}{
		{
			name: "sts assume-role pretty-printed",
			in: "{\n" +
				"  \"Credentials\": {\n" +
				"    \"AccessKeyId\": \"" + asiaKey + "\",\n" +
				"    \"SecretAccessKey\": \"" + secretKey + "\",\n" +
				"    \"SessionToken\": \"" + sessionToken + "\",\n" +
				"    \"Expiration\": \"2026-11-07T12:00:00Z\"\n" +
				"  }\n" +
				"}",
		},
		{
			name: "sts single-line compact",
			in:   "{\"Credentials\":{\"AccessKeyId\":\"" + asiaKey + "\",\"SecretAccessKey\":\"" + secretKey + "\",\"SessionToken\":\"" + sessionToken + "\",\"Expiration\":\"2026-11-07T12:00:00Z\"}}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(Redact([]byte(tc.in)))

			if strings.Contains(got, asiaKey) {
				t.Errorf("AccessKeyId secret leaked: %s", got)
			}
			if strings.Contains(got, secretKey) {
				t.Errorf("SecretAccessKey leaked: %s", got)
			}
			if strings.Contains(got, sessionToken) {
				t.Errorf("SessionToken leaked: %s", got)
			}

			if !strings.Contains(got, `"AccessKeyId": "`+RedactedMark+`"`) &&
				!strings.Contains(got, `"AccessKeyId":"`+RedactedMark+`"`) {
				t.Errorf("expected AccessKeyId to be [REDACTED], got: %s", got)
			}
			if !strings.Contains(got, `"SecretAccessKey": "`+RedactedMark+`"`) &&
				!strings.Contains(got, `"SecretAccessKey":"`+RedactedMark+`"`) {
				t.Errorf("expected SecretAccessKey to be [REDACTED], got: %s", got)
			}
			if !strings.Contains(got, `"SessionToken": "`+RedactedMark+`"`) &&
				!strings.Contains(got, `"SessionToken":"`+RedactedMark+`"`) {
				t.Errorf("expected SessionToken to be [REDACTED], got: %s", got)
			}

			// Verify normal non-secret fields are preserved.
			if !strings.Contains(got, "2026-11-07T12:00:00Z") {
				t.Errorf("Expiration field lost: %s", got)
			}
			var parsed map[string]any
			if err := json.Unmarshal([]byte(got), &parsed); err != nil {
				t.Errorf("redacted output is not valid JSON: %v\nOutput: %s", err, got)
			}
		})
	}
}

func TestRedact_AWSSTSCLIOutput(t *testing.T) {
	asiaKey := "ASIA" + "IOSFODNN7EXAMPLE"
	secretKey := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	sessionToken := "AQoDYXdzEJr" + "1111111111111111EXAMPLETOKEN"

	tests := []struct {
		name     string
		in       string
		wantKept string
	}{
		{
			name:     "sts cli text format",
			in:       "CREDENTIALS\t" + asiaKey + "\t2026-11-07T12:00:00Z\t" + secretKey + "\t" + sessionToken,
			wantKept: "2026-11-07T12:00:00Z",
		},
		{
			name: "sts cli table vertical format",
			in: "| AccessKeyId     | " + asiaKey + " |\n" +
				"| Expiration      | 2026-11-07T12:00:00Z     |\n" +
				"| SecretAccessKey | " + secretKey + " |\n" +
				"| SessionToken    | " + sessionToken + " |",
			wantKept: "2026-11-07T12:00:00Z",
		},
		{
			name: "sts cli key-value assignments",
			in: "AccessKeyId = " + asiaKey + "\n" +
				"Expiration: 2026-11-07T12:00:00Z\n" +
				"SecretAccessKey: " + secretKey + "\n" +
				"SessionToken: " + sessionToken,
			wantKept: "2026-11-07T12:00:00Z",
		},
		{
			name:     "bare ASIA key in log line",
			in:       "Loaded temporary credentials for " + asiaKey + " from cache",
			wantKept: "Loaded temporary credentials for",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(Redact([]byte(tc.in)))

			if strings.Contains(got, asiaKey) {
				t.Errorf("AccessKeyId leaked: %s", got)
			}
			if strings.Contains(got, secretKey) {
				t.Errorf("SecretAccessKey leaked: %s", got)
			}
			if strings.Contains(got, sessionToken) {
				t.Errorf("SessionToken leaked: %s", got)
			}

			if !strings.Contains(got, RedactedMark) {
				t.Errorf("expected [REDACTED] in output, got: %s", got)
			}
			if !strings.Contains(got, tc.wantKept) {
				t.Errorf("expected non-secret content %q preserved, got: %s", tc.wantKept, got)
			}
		})
	}
}

func TestRedact_AWSSSMJSON(t *testing.T) {
	secretVal := "super-secret-password-123"

	tests := []struct {
		name       string
		in         string
		checkJSON  bool
		wantFields []string
	}{
		{
			name: "ssm get-parameter json",
			in: "{\n" +
				"  \"Parameter\": {\n" +
				"    \"Name\": \"/production/db/password\",\n" +
				"    \"Type\": \"SecureString\",\n" +
				"    \"Value\": \"" + secretVal + "\",\n" +
				"    \"Version\": 1,\n" +
				"    \"ARN\": \"arn:aws:ssm:us-east-1:123456789012:parameter/production/db/password\",\n" +
				"    \"DataType\": \"text\"\n" +
				"  }\n" +
				"}",
			checkJSON:  true,
			wantFields: []string{"/production/db/password", "SecureString", "arn:aws:ssm:us-east-1:123456789012:parameter/production/db/password"},
		},
		{
			name: "ssm get-parameters json array",
			in: "{\n" +
				"  \"Parameters\": [\n" +
				"    {\n" +
				"      \"Name\": \"/app/secret1\",\n" +
				"      \"Type\": \"SecureString\",\n" +
				"      \"Value\": \"" + secretVal + "-one\",\n" +
				"      \"Version\": 2\n" +
				"    },\n" +
				"    {\n" +
				"      \"Name\": \"/app/secret2\",\n" +
				"      \"Type\": \"String\",\n" +
				"      \"Value\": \"" + secretVal + "-two\",\n" +
				"      \"Version\": 3\n" +
				"    }\n" +
				"  ],\n" +
				"  \"InvalidParameters\": []\n" +
				"}",
			checkJSON:  true,
			wantFields: []string{"/app/secret1", "/app/secret2", "InvalidParameters"},
		},
		{
			name:       "ssm compact single-line json",
			in:         "{\"Parameter\":{\"Name\":\"/my/secret\",\"Type\":\"SecureString\",\"Value\":\"" + secretVal + "\",\"Version\":1}}",
			checkJSON:  true,
			wantFields: []string{"/my/secret", "SecureString"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(Redact([]byte(tc.in)))

			if strings.Contains(got, secretVal) {
				t.Errorf("SSM secret value leaked: %s", got)
			}
			if !strings.Contains(got, RedactedMark) {
				t.Errorf("expected [REDACTED] in output, got: %s", got)
			}
			for _, field := range tc.wantFields {
				if !strings.Contains(got, field) {
					t.Errorf("expected non-secret field %q preserved, got: %s", field, got)
				}
			}
			if tc.checkJSON {
				var parsed map[string]any
				if err := json.Unmarshal([]byte(got), &parsed); err != nil {
					t.Errorf("redacted output is not valid JSON: %v\nOutput: %s", err, got)
				}
			}
		})
	}
}

func TestRedact_AWSSSMCLIOutput(t *testing.T) {
	secretVal := "super-secret-parameter-value"

	tests := []struct {
		name     string
		in       string
		wantKept string
	}{
		{
			name:     "ssm get-parameters text output",
			in:       "PARAMETERS\tarn:aws:ssm:us-east-1:123456789012:parameter/my/param\t2026-11-07T00:00:00.000Z\t1\t/my/param\tSecureString\ttext\t" + secretVal,
			wantKept: "/my/param\tSecureString\ttext",
		},
		{
			name:     "ssm get-parameter text output",
			in:       "arn:aws:ssm:us-east-1:123456789012:parameter/my/param\t2026-11-07T00:00:00.000Z\t1\t/my/param\tSecureString\ttext\t" + secretVal,
			wantKept: "/my/param\tSecureString\ttext",
		},
		{
			name: "ssm vertical table output",
			in: "| Name     | /my/secret/param |\n" +
				"| Type     | SecureString     |\n" +
				"| Value    | " + secretVal + " |\n" +
				"| Version  | 1                |",
			wantKept: "/my/secret/param",
		},
		{
			name:     "ssm horizontal table row",
			in:       "|| arn:aws:ssm:... | /my/param | SecureString | " + secretVal + " | 1 ||",
			wantKept: "/my/param",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(Redact([]byte(tc.in)))

			if strings.Contains(got, secretVal) {
				t.Errorf("SSM CLI secret leaked: %s", got)
			}
			if !strings.Contains(got, RedactedMark) {
				t.Errorf("expected [REDACTED], got: %s", got)
			}
			if !strings.Contains(got, tc.wantKept) {
				t.Errorf("expected non-secret content %q preserved, got: %s", tc.wantKept, got)
			}
		})
	}
}

func TestRedact_AWSEnvironment(t *testing.T) {
	sessionToken := "AQoDYXdzEJr" + "SESSIONTOKEN123456789"
	secretKey := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	asiaKey := "ASIA" + "IOSFODNN7EXAMPLE"

	cases := []struct {
		name     string
		in       string
		want     string
		leaks    []string
		wantKept string
	}{
		{
			name:     "AWS_SESSION_TOKEN unquoted",
			in:       "AWS_SESSION_TOKEN=" + sessionToken,
			want:     "AWS_SESSION_TOKEN=[REDACTED]",
			leaks:    []string{sessionToken},
			wantKept: "AWS_SESSION_TOKEN=",
		},
		{
			name:     "export AWS_SESSION_TOKEN double quoted",
			in:       "export AWS_SESSION_TOKEN=\"" + sessionToken + "\"",
			want:     "export AWS_SESSION_TOKEN=\"[REDACTED]\"",
			leaks:    []string{sessionToken},
			wantKept: "export AWS_SESSION_TOKEN=",
		},
		{
			name:     "AWS_SECRET_ACCESS_KEY unquoted",
			in:       "AWS_SECRET_ACCESS_KEY=" + secretKey,
			want:     "AWS_SECRET_ACCESS_KEY=[REDACTED]",
			leaks:    []string{secretKey},
			wantKept: "AWS_SECRET_ACCESS_KEY=",
		},
		{
			name:     "export AWS_SECRET_ACCESS_KEY single quoted",
			in:       "export AWS_SECRET_ACCESS_KEY='" + secretKey + "'",
			want:     "export AWS_SECRET_ACCESS_KEY='[REDACTED]'",
			leaks:    []string{secretKey},
			wantKept: "export AWS_SECRET_ACCESS_KEY=",
		},
		{
			name:     "AWS_ACCESS_KEY_ID unquoted",
			in:       "AWS_ACCESS_KEY_ID=" + asiaKey,
			want:     "AWS_ACCESS_KEY_ID=[REDACTED]",
			leaks:    []string{asiaKey},
			wantKept: "AWS_ACCESS_KEY_ID=",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(Redact([]byte(c.in)))
			for _, leak := range c.leaks {
				if strings.Contains(got, leak) {
					t.Errorf("secret leaked: %s", got)
				}
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestRedact_PreservesNormalNonSecretJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{
			name: "json with non-secret Value field outside SSM",
			in: "{\n" +
				"  \"Metric\": {\n" +
				"    \"MetricName\": \"CPUUtilization\",\n" +
				"    \"Value\": \"42.5\",\n" +
				"    \"Unit\": \"Percent\"\n" +
				"  }\n" +
				"}",
		},
		{
			name: "json config with Value string",
			in:   "{\"Config\":{\"Setting\":\"theme\",\"Value\":\"dark\"}}",
		},
		{
			name: "generic json with multiple fields",
			in: "{\n" +
				"  \"status\": \"success\",\n" +
				"  \"count\": 10,\n" +
				"  \"items\": [\"a\", \"b\"],\n" +
				"  \"name\": \"AccessKeyIdentifier\"\n" +
				"}",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(Redact([]byte(c.in)))
			if got != c.in {
				t.Errorf("normal non-secret JSON modified:\nGot:  %s\nWant: %s", got, c.in)
			}
			var parsed map[string]any
			if err := json.Unmarshal([]byte(got), &parsed); err != nil {
				t.Errorf("output is not valid JSON: %v", err)
			}
		})
	}
}
