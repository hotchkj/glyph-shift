# Linting

## Principles

- Keep linting enabled; justify and comment any suppression
- Match CI quality gate to local quality gate exactly — CI invokes the canonical gate, not ad-hoc reimplementations
- Eliminate dead code before commit

## Desired Checks

- Dead code detection
- Unused conditionals and ineffectual assignments
- Static analysis
- Cognitive and cyclomatic complexity
- Security scanning
- Function, file, and line length limits
- Test execution time limits
- IO and timing bans in unit tests

## Metrics

- Keep CRAP score under 8
- Maintain coverage above 90%
- Split files with more than 50 mutation sites by theme (move at least 30%)
