# Bench OpenRouter: small synthetic coding tasks (2026-10-20)

- Tasks: 10 synthetic tasks (`tasks/`), 3 repetitions each, 30 calls per model, 90 calls total.
- Settings: temperature 0, max_tokens 1500, OpenAI-compatible chat endpoint `https://openrouter.ai/api/v1`.
- Pass = the returned code passes the task's assert-based `tests.py` in a fresh subprocess with a 10 s timeout.
- Prices fetched from the public OpenRouter model list at 2026-10-20T19:55+00:00 (USD per 1M tokens):
  - `deepseek/deepseek-v4.1-flash-1017`: input 0.1500, output 0.6000
  - `minimax/minimax-m3.1-flash:free`: input 0.0000, output 0.0000
  - `anthropic/claude-haiku-4.5`: input 1.0000, output 5.0000
- Cost per 10 passing tasks = total cost of all calls of the model (failures included) divided by passes, times 10.

## Per model

| Model | Pass | Median latency (ms) | Mean input tok | Mean output tok | API errors | Cost per 10 passes |
|---|---|---|---|---|---|---|
| `deepseek/deepseek-v4.1-flash-1017` | 24/30 (80%) | 3677 | 147 | 715 | 0 | $0.0056 |
| `minimax/minimax-m3.1-flash:free` | 18/30 (60%) | 11279 | 327 | 884 | 0 | $0.0000 |
| `anthropic/claude-haiku-4.5` | 24/30 (80%) | 3006 | 147 | 387 | 0 | $0.0260 |

## Pass count per task (out of 3)

| Task | `deepseek/deepseek-v4.1-flash-1017` | `minimax/minimax-m3.1-flash:free` | `anthropic/claude-haiku-4.5` |
|---|---|---|---|
| 01_reverse_words | 2 | 1 | 0 |
| 02_binary_search_fix | 3 | 3 | 3 |
| 03_parse_iso8601 | 2 | 0 | 3 |
| 04_snake_case_keys | 3 | 1 | 0 |
| 05_merge_intervals | 3 | 3 | 3 |
| 06_call_with_retry | 3 | 3 | 3 |
| 07_csv_row | 3 | 3 | 3 |
| 08_token_bucket | 2 | 1 | 3 |
| 09_json_get | 0 | 0 | 3 |
| 10_safe_int_div | 3 | 3 | 3 |

## Failures

- `anthropic/claude-haiku-4.5` / 01_reverse_words / rep 1: AssertionError
- `deepseek/deepseek-v4.1-flash-1017` / 01_reverse_words / rep 1: SyntaxError: invalid syntax
- `minimax/minimax-m3.1-flash:free` / 01_reverse_words / rep 1: ImportError: cannot import name 'reverse_words' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmp5dygzyax/candidate.py)
- `anthropic/claude-haiku-4.5` / 01_reverse_words / rep 2: AssertionError
- `minimax/minimax-m3.1-flash:free` / 01_reverse_words / rep 2: ImportError: cannot import name 'reverse_words' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmpx0l7jjq1/candidate.py)
- `anthropic/claude-haiku-4.5` / 01_reverse_words / rep 3: AssertionError
- `minimax/minimax-m3.1-flash:free` / 03_parse_iso8601 / rep 1: ImportError: cannot import name 'parse_iso8601' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmprli6puph/candidate.py)
- `minimax/minimax-m3.1-flash:free` / 03_parse_iso8601 / rep 2: ImportError: cannot import name 'parse_iso8601' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmpcf2v06hh/candidate.py)
- `deepseek/deepseek-v4.1-flash-1017` / 03_parse_iso8601 / rep 3: ImportError: cannot import name 'parse_iso8601' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmp_mzupb8x/candidate.py)
- `minimax/minimax-m3.1-flash:free` / 03_parse_iso8601 / rep 3: ImportError: cannot import name 'parse_iso8601' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmp073or3zq/candidate.py)
- `anthropic/claude-haiku-4.5` / 04_snake_case_keys / rep 1: ModuleNotFoundError: No module named 'anthropic'
- `minimax/minimax-m3.1-flash:free` / 04_snake_case_keys / rep 1: ImportError: cannot import name 'to_snake_case_keys' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmpqqmgobgi/candidate.py)
- `anthropic/claude-haiku-4.5` / 04_snake_case_keys / rep 2: ModuleNotFoundError: No module named 'anthropic'
- `minimax/minimax-m3.1-flash:free` / 04_snake_case_keys / rep 2: SyntaxError: invalid syntax
- `anthropic/claude-haiku-4.5` / 04_snake_case_keys / rep 3: ModuleNotFoundError: No module named 'anthropic'
- `minimax/minimax-m3.1-flash:free` / 08_token_bucket / rep 1: ImportError: cannot import name 'TokenBucket' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmp6o38yr4b/candidate.py)
- `deepseek/deepseek-v4.1-flash-1017` / 08_token_bucket / rep 2: ImportError: cannot import name 'TokenBucket' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmptr7ug0dz/candidate.py)
- `minimax/minimax-m3.1-flash:free` / 08_token_bucket / rep 3: ImportError: cannot import name 'TokenBucket' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmp4ttl87x7/candidate.py)
- `deepseek/deepseek-v4.1-flash-1017` / 09_json_get / rep 1: AssertionError
- `minimax/minimax-m3.1-flash:free` / 09_json_get / rep 1: ImportError: cannot import name 'json_get' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmpvj2l9hzw/candidate.py)
- `deepseek/deepseek-v4.1-flash-1017` / 09_json_get / rep 2: SyntaxError: invalid syntax
- `minimax/minimax-m3.1-flash:free` / 09_json_get / rep 2: ImportError: cannot import name 'json_get' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmpfmfpugk2/candidate.py)
- `deepseek/deepseek-v4.1-flash-1017` / 09_json_get / rep 3: ImportError: cannot import name 'json_get' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmp28hyql3s/candidate.py)
- `minimax/minimax-m3.1-flash:free` / 09_json_get / rep 3: ImportError: cannot import name 'json_get' from 'candidate' (/var/folders/x7/gv26pg6x13z1t6pfcgxhsgwc0000gn/T/tmphe0q96qy/candidate.py)

## What this does not show

- n is small: 10 synthetic tasks x 3 repetitions per model. The pass rates have wide intervals and say little about the true rate.
- The tasks are small, single-function problems with asserts written by the author. They are not real repository work, multi-file changes, or long contexts.
- Temperature 0 is not fully deterministic on hosted providers; repetitions show variance only partly.
- Prices are list prices at the fetch time. DeepSeek lists time-of-day (UTC) overrides; the base price is used here.
- Free tiers may be rate-limited or change routing; latency includes queueing at the provider and network time.
- Data retention, privacy terms and zero-data-retention (ZDR) status of each provider are not evaluated here.
- Errors are reported as they occurred; none were retried.
- Output cap: 1500 max_tokens. 17 responses stopped at the cap (finish_reason=length) and 0 outputs were empty; these count as failures and are a confound for the models that produce long reasoning output.
- Task 02 was run twice. The first run used a checker with a wrong expectation (it asserted that present values return -1), so all 9 of its rows failed for the checker's reason. The checker was fixed and only task 02 was rerun; the discarded rows are kept in results_discarded_task02_checker_bug.jsonl.
