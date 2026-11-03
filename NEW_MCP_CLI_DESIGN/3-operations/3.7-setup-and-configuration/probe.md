#### `probe` (tool + CLI)

This asks one model one question and reports what came back. Each call is a paid model call.

- **Arguments**: one of `decision`, `text`, `scoring` (default) or `model` (a reference); `prompt`;
  `timeout`.
- **Result**: `provider`, `provider-rule`, `provider-layer`, `selection`, `model`, `temperature`,
  `thinking`, `first-token-ms`, `wall-ms`, `finish-reason`, `reply`, `usage`.
- **Refusals**: more than one selector (usage); a selection that does not resolve, naming the
  key, flag or setting at fault.
