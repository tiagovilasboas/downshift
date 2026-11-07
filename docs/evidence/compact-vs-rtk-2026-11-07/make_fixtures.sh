#!/bin/sh
# Builds the fixture projects for the compact-vs-rtk comparison.
# Downshift by Tiago Vilas Boas - https://github.com/tiagovilasboas/downshift
# Usage: make_fixtures.sh <scratch-dir> <path-to-a-git-checkout-of-downshift>
# Needs: go, python3 with pytest on PATH, cargo, node and npm (registry.npmjs.org), git.
set -eu
S=$1; REPO=$2; F=$S/fix
rm -rf "$F"; mkdir -p "$F/gopass" "$F/gofail" "$F/py" "$F/rust" "$F/ts"

# Go: 12 passing packages (24 tests)
cd "$F/gopass"; printf 'module fixpass\n\ngo 1.21\n' > go.mod
for i in $(seq 1 12); do
  mkdir -p p$i
  printf 'package p%s\n\nfunc Add(a, b int) int { return a + b }\n' $i > p$i/p.go
  printf 'package p%s\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal("bad")\n\t}\n}\n\nfunc TestAdd2(t *testing.T) {\n\tif Add(2, 2) != 4 {\n\t\tt.Fatal("bad")\n\t}\n}\n' $i > p$i/p_test.go
done

# Go: one ok package, one with three failing tests, one that panics
cd "$F/gofail"; printf 'module fixfail\n\ngo 1.21\n' > go.mod
mkdir okpkg failpkg panicpkg
printf 'package okpkg\n\nfunc One() int { return 1 }\n' > okpkg/o.go
printf 'package okpkg\n\nimport "testing"\n\nfunc TestOne(t *testing.T) {\n\tif One() != 1 {\n\t\tt.Fatal("x")\n\t}\n}\n' > okpkg/o_test.go
printf 'package failpkg\n\nfunc Mul(a, b int) int { return a + b }\n' > failpkg/f.go
printf 'package failpkg\n\nimport "testing"\n\nfunc TestMulTwoByThree(t *testing.T) {\n\tif got := Mul(2, 3); got != 6 {\n\t\tt.Errorf("Mul(2,3) = %%d, want 6", got)\n\t}\n}\n\nfunc TestMulFourByFive(t *testing.T) {\n\tif got := Mul(4, 5); got != 20 {\n\t\tt.Errorf("Mul(4,5) = %%d, want 20", got)\n\t}\n}\n\nfunc TestMulZero(t *testing.T) {\n\tif Mul(0, 9) != 0 {\n\t\tt.Error("zero case broken")\n\t}\n}\n' > failpkg/f_test.go
printf 'package panicpkg\n\nfunc Idx(s []int, i int) int { return s[i] }\n' > panicpkg/p.go
printf 'package panicpkg\n\nimport "testing"\n\nfunc TestIdxOutOfRange(t *testing.T) {\n\t_ = Idx([]int{1, 2, 3}, 7)\n}\n' > panicpkg/p_test.go

# Python: 40 passing tests; 30 passing, 3 failing assertions, 1 raising
cd "$F/py"; printf 'def add(a, b):\n    return a + b\n' > calc.py
{ echo 'import calc, pytest'; for i in $(seq 1 40); do printf 'def test_add_%s():\n    assert calc.add(%s, 1) == %s\n\n' $i $i $((i+1)); done; } > test_pass.py
{ echo 'import calc, pytest'
  for i in $(seq 1 30); do printf 'def test_ok_%s():\n    assert calc.add(%s, 1) == %s\n\n' $i $i $((i+1)); done
  for i in 1 2 3; do printf 'def test_broken_%s():\n    values = [calc.add(x, 1) for x in range(5)]\n    assert values == [2, 3, 4, 5, %s]\n\n' $i $((10+i)); done
  printf 'def test_raises():\n    raise ValueError("config missing: db_url")\n'; } > test_fail.py

# Rust: 12 passing tests; a second crate with two failing assertions and a panic
cd "$F/rust"; cargo init --lib --name fixrust -q
{ echo 'pub fn add(a: i32, b: i32) -> i32 { a + b }'; echo 'pub fn mul(a: i32, b: i32) -> i32 { a + b }'
  echo '#[cfg(test)] mod tests { use super::*;'
  for i in $(seq 1 12); do echo "#[test] fn add_$i() { assert_eq!(add($i, 1), $((i+1))); }"; done; echo '}'; } > src/lib.rs
mkdir -p "$F/rustfail/tests"; cp Cargo.toml "$F/rustfail/"; cp -r src "$F/rustfail/"
printf 'use fixrust::*;\n#[test] fn mul_works() { assert_eq!(mul(3, 4), 12, "multiplication of 3 by 4"); }\n#[test] fn mul_zero() { assert_eq!(mul(0, 5), 0); }\n#[test] fn idx_panics() { let v = vec![1, 2, 3]; let _ = v[7]; }\n' > "$F/rustfail/tests/fail.rs"

# TypeScript: four type errors; vitest with 12 passing files and one file with 2 failing tests
cd "$F/ts"; printf '{"name":"fixts","private":true,"type":"module"}\n' > package.json
npm i -D typescript vitest@2 --no-audit --no-fund >/dev/null 2>&1
printf '{"compilerOptions":{"strict":true,"noEmit":true,"target":"es2022","module":"esnext","moduleResolution":"bundler","types":[]},"include":["src"]}\n' > tsconfig.json
mkdir -p bad/src vt; cp tsconfig.json bad/; ln -s ../node_modules bad/node_modules
printf 'const a: number = "one"\nexport function g(x: string): number { return x }\nexport const h = missing + 1\nexport function k(n: number) { return n.toUpperCase() }\n' > bad/src/bad.ts
for i in $(seq 1 12); do printf 'import {test,expect} from "vitest"\ntest("pass %s",()=>{expect(%s+1).toBe(%s)})\n' $i $i $((i+1)) > vt/ok$i.test.ts; done
printf 'import {test,expect} from "vitest"\ntest("adds correctly",()=>{expect(2+2).toBe(5)})\ntest("fetches config",()=>{throw new Error("config missing: api_key")})\n' > vt/fail.test.ts

# git: a checkout with a staged rename, two edits and two untracked files
git clone -q "$REPO" "$F/gitrepo"; cd "$F/gitrepo"
git config user.email bench@example.invalid; git config user.name bench
sed -i 's/ccr/CCR/' README.md; echo "// touch" >> internal/ccr/store.go
echo x > untracked1.txt; echo y > untracked2.txt; git mv CHANGELOG.md CHANGELOG.old.md

# 150 small files plus 40 markdown files in a subdirectory
mkdir -p "$F/many/sub/a"; cd "$F/many"
for i in $(seq 1 150); do echo "line $i TODO fix" > file$i.txt; done
for i in $(seq 1 40); do echo "TODO item $i" > sub/a/n$i.md; done
echo "fixtures in $F"
