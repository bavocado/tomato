# Implementation Design

## 1. File and Package Plan
- Modify `cmd/commands.go`: update `NewConfigCmd()` output.
- Modify `cmd/init_test.go` or add `cmd/config_test.go`: test masked output.

## 2. Public Types / Functions / CLI Flags
No new public types or flags.

## 3. Step-by-Step Algorithm
1. In `NewConfigCmd()`, after printing model routing, print `Anthropic:` section.
2. Print `base_url`: configured if non-empty.
3. Print `auth_token`: configured with masked prefix if non-empty.
4. Print `model`: configured if non-empty.
5. Preserve existing API key env var status output.

## 4. Data Structures
Use existing `cfg.Anthropic`.

## 5. Migration / Backward Compatibility
Backward compatible.

## 6. Test Plan
- `TestConfigCommandShowsAnthropicYamlStatus`
- `TestConfigCommandMasksShortAnthropicToken`

## 7. Rollout / Verification
Run `go test ./cmd -run TestConfig` and `go test ./...`.
