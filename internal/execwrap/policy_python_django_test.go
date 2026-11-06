// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestFamilyOf_PythonQualityAndDjango(t *testing.T) {
	cases := []struct {
		command string
		want    Family
	}{
		{"flake8", FamilyFlake8},
		{"flake8 src/", FamilyFlake8},
		{"pylint app", FamilyPylint},
		{"bandit -r src", FamilyBandit},
		{"python -m pytest", FamilyPytest},
		{"python3 -m pytest -x tests/", FamilyPytest},
		{"python3.12 -m pytest", FamilyPytest},
		{"python -m unittest discover", FamilyUnittest},
		{"python3 -m mypy src", FamilyMypy},
		{"python -m flake8", FamilyFlake8},
		{"python -m pylint app", FamilyPylint},
		{"tox", FamilyTox},
		{"tox -e py312", FamilyTox},
		{"tox --recreate", FamilyTox},
		{"nox -s tests", FamilyNox},
		{"coverage run -m pytest", FamilyCoverage},
		{"coverage run --branch -m unittest", FamilyCoverage},
		{"coverage report", FamilyCoverage},
		{"python manage.py test", FamilyDjangoTest},
		{"python3 manage.py test shop --keepdb", FamilyDjangoTest},
		{"python manage.py check", FamilyDjangoCheck},
		{"python manage.py check --deploy", FamilyDjangoCheck},
		{"python manage.py makemigrations --check --dry-run --noinput", FamilyDjangoMigrations},
		{"python manage.py makemigrations --dry-run --noinput", FamilyDjangoMigrations},
		{"python manage.py showmigrations", FamilyDjangoMigrations},
		{"python manage.py migrate --plan", FamilyDjangoMigrations},
		{"python ./manage.py test", FamilyDjangoTest},
		{"django-admin check", FamilyDjangoCheck},
		{"django-admin test", FamilyDjangoTest},
		// refused: run code, install, serve, or write
		{"python", FamilyNone},
		{"python app.py", FamilyNone},
		{"python -c print(1)", FamilyNone},
		{"python -m pip install requests", FamilyNone},
		{"python -m http.server", FamilyNone},
		{"python -m venv .venv", FamilyNone},
		{"python -u -m pytest", FamilyNone},
		{"python -m", FamilyNone},
		{"python4 -m pytest", FamilyNone},
		{"python3. -m pytest", FamilyNone},
		{"tox devenv -e py312", FamilyNone},
		{"tox exec -e py312 -- rm", FamilyNone},
		{"coverage run app.py", FamilyNone},
		{"coverage run -m http.server", FamilyNone},
		{"coverage run", FamilyNone},
		{"coverage erase", FamilyNone},
		{"coverage", FamilyNone},
		{"python manage.py runserver", FamilyNone},
		{"python manage.py shell", FamilyNone},
		{"python manage.py dbshell", FamilyNone},
		{"python manage.py migrate", FamilyNone},
		{"python manage.py migrate app 0001", FamilyNone},
		{"python manage.py makemigrations", FamilyNone},
		{"python manage.py makemigrations --merge --dry-run", FamilyNone},
		{"python manage.py makemigrations --empty --check app", FamilyNone},
		{"python manage.py flush", FamilyNone},
		{"python manage.py loaddata fixture.json", FamilyNone},
		{"python manage.py createsuperuser", FamilyNone},
		{"python manage.py collectstatic", FamilyNone},
		{"python manage.py", FamilyNone},
		{"python other.py test", FamilyNone},
		{"django-admin startproject site", FamilyNone},
		{"django-admin", FamilyNone},
	}
	for _, c := range cases {
		if got := FamilyOf(strings.Fields(c.command)); got != c.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}
