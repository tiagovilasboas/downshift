import unittest
from compare import compare


def rows():
    return [{"case": "C01", "arm": arm, "repetition": rep,
             "accepted": True, "critical_failure": False}
            for arm in ("A", "B") for rep in range(1, 4)]


class CompareTest(unittest.TestCase):
    def test_missing_is_incomplete(self):
        self.assertEqual(compare(["C01"], rows()[:-1])["verdict"], "incomplete")

    def test_acceptance_regression_blocks(self):
        data = rows()
        data[-1]["accepted"] = False
        self.assertEqual(compare(["C01"], data)["verdict"], "regression")

    def test_candidate_critical_failure_blocks(self):
        data = rows()
        data[-1]["critical_failure"] = True
        self.assertEqual(compare(["C01"], data)["verdict"], "regression")

    def test_complete_equal_is_limited_result(self):
        self.assertEqual(compare(["C01"], rows())["verdict"], "no_observed_regression")

    def test_equal_failures_do_not_count_as_preserved_capability(self):
        data = rows()
        for row in data:
            row["accepted"] = False
        self.assertEqual(compare(["C01"], data)["verdict"], "baseline_failed")

    def test_duplicate_and_missing_assessments_rejected(self):
        data = rows()
        with self.assertRaises(ValueError):
            compare(["C01"], data + data[:1])
        del data[0]["accepted"]
        with self.assertRaises(ValueError):
            compare(["C01"], data)


if __name__ == "__main__":
    unittest.main()
