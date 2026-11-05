#### Writes leave nothing stale

Every write batch is followed by the rebuild of the derived views (strengths, summary pages and sheets), inside its all-or-nothing; no flag skips it. The rebuild does not repair or convert the KB. The writes of `score` ([§8](../score/writing-the-results.md)) and of the build ([§5](../../5-build-behaviour/index.md)) are followed by it the same way.

