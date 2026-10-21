# Adaptive Tier Memory

Downshift learns from outcomes to improve routing over time. When a task
fails on a lower tier, the system remembers and routes similar tasks higher.

## How It Works

### Outcome Tracking

After a spawn completes, Downshift observes the outcome:

```
Spawn completes → Check outcome → Update memory
                       ↓
              Pass: tier was sufficient
              Fail: tier was insufficient
```

### Memory Key

Similar prompts are grouped by a semantic key:

```
"rename userId to userID" → key: rename_variable
"rename the field email"  → key: rename_variable
```

Both map to the same memory entry.

### Tier Adjustment

When a task fails on SMALL tier:

```
Before: SIMPLE → SMALL (default)
Fail observed
After:  SIMPLE → MID (for this key)
```

The adjustment persists until evidence says SMALL works again.

## Memory Storage

Adaptive memory is stored in `~/.downshift/adapt-memory.json`:

```json
{
  "rename_variable": {
    "floor": "SMALL",
    "ceiling": "MID",
    "observations": 12,
    "last_fail_tier": null
  },
  "implement_oauth": {
    "floor": "MID",
    "ceiling": "FRONTIER",
    "observations": 3,
    "last_fail_tier": "MID"
  }
}
```

## Commands

### View Memory

```bash
downshift config get adapt.memory
```

### Clear Memory

```bash
downshift config set adapt.memory {}
```

### Manual Feedback

Record that a task failed:

```bash
downshift feedback --task="implement OAuth" --outcome=failed --tier=MID
```

Record that a task succeeded:

```bash
downshift feedback --task="implement OAuth" --outcome=passed --tier=FRONTIER
```

## Evaluation

### Contrafactual Analysis

```bash
downshift eval-adapt --period=30d
```

Shows what would have happened with different tier assignments:

```
Memory Key          Obs  Small  Mid  Frontier  Adjusted
rename_variable     45   98%    100% 100%      SMALL (correct)
implement_feature   23   45%    92%  100%      MID (improved)
security_review     8    0%     20%  95%       FRONTIER (risk)
```

### Outcome Evaluation

```bash
downshift eval-outcome --period=7d
```

Compares baseline (no adapt) vs adapted routing.

## How Adaptation Helps

### Without Adaptation

```
Day 1: "implement OAuth" → MEDIUM → MID tier → fails
Day 2: "implement OAuth" → MEDIUM → MID tier → fails
Day 3: "implement OAuth" → MEDIUM → MID tier → fails
```

Same classification, same failure.

### With Adaptation

```
Day 1: "implement OAuth" → MEDIUM → MID tier → fails
        Memory: implement_oauth needs FRONTIER
Day 2: "implement OAuth" → MEDIUM → FRONTIER tier → passes
Day 3: "add OAuth scope"  → MEDIUM → FRONTIER tier → passes
```

The system learns and applies knowledge to similar tasks.

## Configuration

### Enable/Disable

```bash
# Disable adaptation (classification only)
export DOWNSHIFT_ADAPT=0

# Enable (default)
export DOWNSHIFT_ADAPT=1
```

### Memory Decay

Old observations decay over time:

```bash
downshift config set adapt.decay_days 30
```

Observations older than 30 days have reduced weight.

### Minimum Observations

Require N observations before adapting:

```bash
downshift config set adapt.min_observations 3
```

Prevents one-off failures from permanently elevating a key.

## Privacy

Adaptive memory stores:
- Semantic keys (not full prompts)
- Tier observations (pass/fail)
- Timestamps

It does **not** store:
- Actual prompt text
- Code content
- File paths
