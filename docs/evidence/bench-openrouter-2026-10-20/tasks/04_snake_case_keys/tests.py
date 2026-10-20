from candidate import to_snake_case_keys
assert to_snake_case_keys({"userName": 1}) == {"user_name": 1}
assert to_snake_case_keys({"HelloWorld2Go": "x"}) == {"hello_world2_go": "x"}
assert to_snake_case_keys({"Nested-Key": None}) == {"nested_key": None}
src = {"userName": {"firstName": [{"helloWorld": 2}, 3, "keepMe"]}, "ok": [1, {"aB": {"cD": True}}]}
out = to_snake_case_keys(src)
assert out == {"user_name": {"first_name": [{"hello_world": 2}, 3, "keepMe"]}, "ok": [1, {"a_b": {"c_d": True}}]}, out
assert src["userName"]["firstName"][0] == {"helloWorld": 2}
assert to_snake_case_keys(5) == 5
assert to_snake_case_keys([]) == []
print("ALL PASS")
