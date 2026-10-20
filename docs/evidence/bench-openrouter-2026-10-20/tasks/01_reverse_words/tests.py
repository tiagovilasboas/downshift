from candidate import reverse_words
assert reverse_words("Hi, there!") == "there, Hi!"
assert reverse_words("one two  three") == "three two  one"
assert reverse_words("") == ""
assert reverse_words("abc") == "abc"
assert reverse_words("a-b.c") == "c-b.a"
assert reverse_words("  lead trail  ") == "  trail lead  "
assert reverse_words("x1 y2") == "y2 x1"
print("ALL PASS")
