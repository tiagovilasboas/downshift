from candidate import safe_int_div
assert safe_int_div(7, 2) == 3
assert safe_int_div(-7, 2) == -3
assert safe_int_div(7, -2) == -3
assert safe_int_div(-7, -2) == 3
assert safe_int_div(0, 5) == 0
assert safe_int_div(10**30, 7) == 10**30 // 7
assert safe_int_div(-(10**30), 7) == -(10**30 // 7)
def expect(exc, *args):
    try:
        safe_int_div(*args)
    except exc:
        return
    raise AssertionError("expected %s for %r" % (exc.__name__, args))
expect(ZeroDivisionError, 1, 0)
expect(TypeError, 1.0, 2)
expect(TypeError, "1", 2)
expect(TypeError, True, 1)
expect(TypeError, 4, None)
print("ALL PASS")
