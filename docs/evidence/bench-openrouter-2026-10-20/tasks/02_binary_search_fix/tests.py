from candidate import binary_search
assert binary_search([], 3) == -1
assert binary_search([5], 5) == 0
assert binary_search([5], 4) == -1
for n in range(1, 40):
    xs = list(range(0, 2 * n, 2))
    for i, v in enumerate(xs):
        assert binary_search(xs, v) == i, (n, v)
    for v in range(-1, 2 * n + 1, 2):
        assert binary_search(xs, v) == -1, (n, v)
    assert binary_search(xs, -1) == -1
    assert binary_search(xs, 2 * n + 5) == -1
print("ALL PASS")
