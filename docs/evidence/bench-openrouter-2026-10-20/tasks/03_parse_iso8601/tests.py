import datetime
from datetime import timedelta
from candidate import parse_iso8601
d = parse_iso8601("2026-10-20T14:30:00Z")
assert d.tzinfo is not None and d.utcoffset() == timedelta(0) and d.hour == 14 and d.minute == 30
d = parse_iso8601("2026-10-20T14:30:00+02:00")
assert d.utcoffset() == timedelta(hours=2) and d.hour == 14
d = parse_iso8601("2026-10-20T14:30:00.250-03:00")
assert d.utcoffset() == timedelta(hours=-3) and d.microsecond == 250000
assert d.astimezone(datetime.timezone.utc).hour == 17
assert isinstance(parse_iso8601("2026-01-01T00:00:00Z"), datetime.datetime)
for bad in ["2026-10-20T14:30:00", "not a date", "2026-13-01T00:00:00Z", ""]:
    try:
        parse_iso8601(bad)
    except ValueError:
        pass
    else:
        raise AssertionError("expected ValueError for " + repr(bad))
print("ALL PASS")
