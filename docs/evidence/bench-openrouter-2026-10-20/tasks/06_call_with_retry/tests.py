from candidate import call_with_retry

def flaky(n_fail, value="ok"):
    state = {"calls": 0}
    def fn():
        state["calls"] += 1
        if state["calls"] <= n_fail:
            raise ValueError("boom")
        return value
    return fn, state

sleeps = []
fn, st = flaky(2)
assert call_with_retry(fn, 5, 0.5, sleeps.append) == "ok"
assert st["calls"] == 3, st
assert sleeps == [0.5, 1.0], sleeps

sleeps = []
fn, st = flaky(99)
try:
    call_with_retry(fn, 3, 1, sleeps.append)
    raise AssertionError("should raise")
except ValueError:
    pass
assert st["calls"] == 3, st
assert sleeps == [1, 2], sleeps

sleeps = []
fn, st = flaky(0)
assert call_with_retry(fn, 3, 1, sleeps.append) == "ok"
assert st["calls"] == 1 and sleeps == []

sleeps = []
fn, st = flaky(1)
try:
    call_with_retry(fn, 1, 1, sleeps.append)
    raise AssertionError("should raise")
except ValueError:
    pass
assert st["calls"] == 1 and sleeps == []
print("ALL PASS")
