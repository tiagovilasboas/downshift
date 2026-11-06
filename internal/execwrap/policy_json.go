// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// DOMAIN: reading one JSON file through jq. It is a file-read family
// (FamilyFileRead): the same run-time checks apply (regular file, not a link,
// no secret-looking name), and the compressor reads the output as JSON.

// jqIdentityArgs reads `. FILE`: the identity filter over one file. Any other
// filter or option can compute, select or write (--arg, -f, --rawfile,
// input_filename, ...), so it is not accepted.
func jqIdentityArgs(args []string) (file string, ok bool) {
	if len(args) != 2 || args[0] != "." {
		return "", false
	}
	return args[1], true
}
