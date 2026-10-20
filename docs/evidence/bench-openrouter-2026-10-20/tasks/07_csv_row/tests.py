from candidate import csv_row
assert csv_row(["a", "b"]) == "a,b"
assert csv_row(["a,b", "c"]) == '"a,b",c'
assert csv_row(['say "hi"']) == '"say ""hi"""'
assert csv_row(["line1\nline2"]) == '"line1\nline2"'
assert csv_row(["x\r"]) == '"x\r"'
assert csv_row([None, 3, 4.5]) == ",3,4.5"
assert csv_row([]) == ""
assert csv_row(["a b"]) == "a b"
assert csv_row(iter(["p", "q"])) == "p,q"
print("ALL PASS")
