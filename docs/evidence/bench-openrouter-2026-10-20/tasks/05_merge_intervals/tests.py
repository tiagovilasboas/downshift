from candidate import merge_intervals
assert merge_intervals([[1, 3], [2, 6], [8, 10], [15, 18]]) == [[1, 6], [8, 10], [15, 18]]
assert merge_intervals([[1, 4], [4, 5]]) == [[1, 5]]
assert merge_intervals([]) == []
assert merge_intervals([[5, 5]]) == [[5, 5]]
assert merge_intervals([[8, 10], [1, 3], [2, 6]]) == [[1, 6], [8, 10]]
assert merge_intervals([[0, 0], [0, 1]]) == [[0, 1]]
assert merge_intervals([[1, 10], [2, 3], [4, 5]]) == [[1, 10]]
src = [[3, 4], [1, 2]]
assert merge_intervals(src) == [[1, 2], [3, 4]]
assert src == [[3, 4], [1, 2]]
print("ALL PASS")
