# PRD: Show Anthropic YAML Configuration Status in `tomato config`

## 1. Problem Statement
`tomato config` currently lists API key status for providers as if all providers use environment variables. Anthropic now runs through the `claude` CLI and gets `base_url`, `auth_token`, and `model` from `tomato.yaml`, so the current output can mislead users.

## 2. Goals and Non-Goals
### Goals
- Show whether `anthropic.base_url`, `anthropic.auth_token`, and `anthropic.model` are configured.
- Mask the Anthropic auth token in output.
- Keep existing OpenAI / GLM / DeepSeek environment key display.

### Non-Goals
- Do not validate whether the token is correct.
- Do not call the `claude` CLI.
- Do not change the config file format.

## 3. Target Users
Developers configuring tomato for the first time with Claude Code / Anthropic models.

## 4. User Stories
- As a tomato user, I want `tomato config` to show whether Anthropic config is set in `tomato.yaml`, so I know whether `tomato spec` can use Claude.
- As a tomato user, I want auth tokens masked, so secrets are not leaked in terminal logs.

## 5. Functional Requirements
- FR-001: `tomato config` must print an `Anthropic:` section.
- FR-002: It must show `base_url` as configured or `not set`.
- FR-003: It must show `auth_token` as masked when present, and `not set` when empty.
- FR-004: It must show `model` as configured or `not set`.
- FR-005: It must preserve existing env var status output for `OPENAI_API_KEY`, `GLM_API_KEY`, `DEEPSEEK_API_KEY`.

## 6. Acceptance Criteria
- Given `anthropic.auth_token: sk-ant-abcdef123456`, when `tomato config` runs, then output includes `auth_token: ✓ configured (sk-ant-a...)` and does not include the full token.
- Given empty `anthropic.auth_token`, when `tomato config` runs, then output includes `auth_token: ✗ not set`.

## 7. Edge Cases and Error States
- Empty token.
- Very short token shorter than 8 characters.
- Empty base URL.
- Empty model.

## 8. Data and State
Reads `tomato.yaml`. Writes nothing.

## 9. Dependencies and Integrations
None.

## 10. Open Questions
None.
