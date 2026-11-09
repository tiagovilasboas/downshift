// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// routeFamily is the domain router behind FamilyOf, before the shape refusals.
func routeFamily(argv []string) Family {
	if len(argv) == 0 {
		return FamilyNone
	}
	if argv[0] == "git" && len(argv) >= 2 {
		return gitFamilyOf(argv[1:])
	}
	if family := frontendFamily(argv); family != FamilyNone {
		return family
	}
	if family := backendFamily(argv); family != FamilyNone {
		return family
	}
	if family := infraFamily(argv); family != FamilyNone {
		return family
	}
	if family := dbFamily(argv); family != FamilyNone {
		return family
	}
	if family := observabilityFamily(argv); family != FamilyNone {
		return family
	}
	if family := toolingFamily(argv); family != FamilyNone {
		return family
	}
	if family := fetchFamily(argv); family != FamilyNone {
		return family
	}
	if family := glabFamily(argv); family != FamilyNone {
		return family
	}
	if family := desktopFamily(argv); family != FamilyNone {
		return family
	}
	return FamilyNone
}
