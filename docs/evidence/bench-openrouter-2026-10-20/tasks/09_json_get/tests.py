from candidate import json_get
data = {"user": {"items": [{"name": "a"}, {"name": "b"}]}, "matrix": [[1, 2], [3, 4]], "n": None, "z": 0}
assert json_get(data, "user.items[1].name") == "b"
assert json_get(data, "user.items[2].name") is None
assert json_get(data, "user.items[2].name", "x") == "x"
assert json_get(data, "user.name.first", "d") == "d"
assert json_get(data, "matrix[1][0]") == 3
assert json_get(data, "") is data
assert json_get(data, "n", "d") is None
assert json_get(data, "z.b", "d") == "d"
assert json_get(data, "user.items[-1].name", "neg") == "neg"
assert json_get([1, 2], "[0]") == 1
assert json_get(data, "user.items") == data["user"]["items"]
assert data["user"]["items"][1] == {"name": "b"}
print("ALL PASS")
