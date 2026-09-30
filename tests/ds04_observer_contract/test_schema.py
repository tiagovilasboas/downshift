import unittest

from schema import SCHEMA_VERSION, validate


def observed(**changes):
    base = {
        "schema_version": SCHEMA_VERSION,
        "evidence_state": "observed",
        "observed_at": "2026-09-30T12:00:00Z",
        "lifecycle": "completed",
        "observer_id_hash": "a" * 64,
        "subject_id_hash": "b" * 64,
        "association_state": "matched",
        "correlation_id": "0123456789abcdef",
    }
    return base | changes


class ObserverContractTests(unittest.TestCase):
    def test_accepts_only_observed_event_with_opaque_ids(self):
        self.assertTrue(validate(observed()))

    def test_rejects_ack_claim_or_forbidden_conversation_data(self):
        self.assertFalse(validate(observed(evidence_state="executor_acknowledged")))
        self.assertFalse(validate(observed(messages=["private transcript"])))

    def test_ambiguous_concurrency_omits_correlation(self):
        self.assertTrue(validate(observed(association_state="ambiguous", correlation_id=None)))
        self.assertFalse(validate(observed(association_state="ambiguous")))

    def test_rejects_raw_or_malformed_identifiers(self):
        self.assertFalse(validate(observed(subject_id_hash="thread-raw")))
        self.assertFalse(validate(observed(correlation_id="guessed title")))
        self.assertFalse(validate(observed(observed_at="private prompt")))


if __name__ == "__main__":
    unittest.main()
