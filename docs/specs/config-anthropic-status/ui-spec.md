# UI Specification

## 1. Surfaces / Screens / Commands
CLI surface: `tomato config`.

## 2. User Flows
User runs `tomato config` after editing `tomato.yaml`.

## 3. Command Behavior
Add output section:

```text
Anthropic:
  base_url: ✓ https://api.anthropic.com
  auth_token: ✓ configured (sk-ant-a...)
  model: ✓ claude-sonnet-4-20250514
```

When missing:

```text
Anthropic:
  base_url: ✗ not set
  auth_token: ✗ not set
  model: ✗ not set
```

## 4. Empty / Loading / Error States
No loading state. Config load errors remain as-is.

## 5. Copy and Terminology
Use `Anthropic`, `base_url`, `auth_token`, `model` exactly as in YAML.

## 6. Accessibility / Usability Notes
Keep terminal output plain text and greppable.
