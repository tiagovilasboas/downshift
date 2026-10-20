from candidate import TokenBucket
t = [0.0]
clock = lambda: t[0]
b = TokenBucket(2, 1, clock)
assert b.allow() is True
assert b.allow() is True
assert b.allow() is False
t[0] = 0.5
assert b.allow() is False
t[0] = 1.0
assert b.allow() is True
assert b.allow() is False
t[0] = 100.0
assert b.allow(2.0) is True
assert b.allow() is False
t[0] = 200.0
c = TokenBucket(3, 1, clock)
assert c.allow(2.5) is True
assert c.allow(1) is False
t[0] = 200.5
assert c.allow(1) is True
print("ALL PASS")
