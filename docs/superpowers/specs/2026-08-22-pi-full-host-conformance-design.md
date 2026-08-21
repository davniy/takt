# Pi Full Host Conformance: дизайн повторяемого evidence-gate

## Статус и цель

Дизайн подтверждён пользователем 2026-08-22. Цель — повторить на одном
финальном bundled extension полный Pi `0.84.1` host-control сценарий
`command → input → tool → completion → recovery`, сохранить redacted evidence
и не включать `strict` по единственному полному PASS.

Проектный шлюз: **READY**. Открытых архитектурных P0 нет; внешний provider и
TUI остаются эксплуатационными зависимостями live-прогона, а не причиной
расширять runtime.

## Границы

В scope входят:

- только bundled Pi host extension и Pi `0.84.1`;
- один disposable trusted workspace и одна managed host session;
- command/input interception, native pre-execution tool deny, streaming
  suppression, final replacement, daemon loss и durable recovery;
- проверка отсутствия mutation marker;
- redacted runbook/evidence и machine-readable compatibility status.

Не входят OpenCode/Qwen adapters, новый test framework, PTY dependency, новый
shell test, публичное Workflow/Config поле, scheduler semantics и изменение
strict host contract.

## Подтверждённая модель

| Объект | Текущий контракт | Свидетельство |
|---|---|---|
| Pi extension | Pinned `@earendil-works/pi-coding-agent@0.84.1`; guarded; five host capabilities declared | `integrations/coding-agent-host-control/pi/`, `internal/tooling/compatibility/compatibility.go` |
| Deterministic boundary | TypeScript smoke проверяет tool deny, streaming/final replacement и daemon fail-closed | `integrations/coding-agent-host-control/contracts/pi-blocking-contract.mts` |
| Live evidence | Отдельные command/input/recovery и tool/completion probes PASS, но не одним финальным suite | `docs/archive/verification/TEST_RESULTS-v0.1.57-2026-08-18.md` |
| Strict policy | Сейчас `enforcement=guarded`, `live_verified=false`, `strict_allowed=false`; missing `full_live_conformance` | `internal/tooling/compatibility/compatibility.go` |

Production extension уже использует native Pi hooks. Поэтому повторяемость
достигается документированным opt-in live runbook поверх существующих entrypoint
и daemon API, без второго executor или автоматизированного TUI framework.

## Live-сценарий

Прогон выполняется последовательно в одном disposable workspace:

1. зафиксировать Pi version, CLI fingerprint и final extension fingerprint;
2. запустить локальный daemon и Pi TUI с production extension;
3. вызвать `/takt` и подтвердить, что preview появляется до main-model output;
4. подтвердить workflow и отправить обычный input; он должен уйти в Takt и не
   вызвать main model;
5. дать модели вызвать отдельный mutating probe tool; native `tool_call` обязан
   вернуть deny, а marker-файл не должен появиться;
6. получить premature assistant final; streaming markdown не должен быть
   показан, finalized message должен стать `TAKT_COMPLETION_BLOCKED`, production
   extension не должен отправлять follow-up;
7. остановить daemon, убедиться, что active session блокирует input/tools
   fail-closed, затем запустить daemon и восстановить ту же durable session через
   `host find`/`/takt-status`;
8. остановить процессы, проверить marker ещё раз и переместить disposable
   workspace в Trash.

Driver extension разрешён только для регистрации probe tool и подачи bounded
prompt. Он не может реализовывать ни один проверяемый guard, менять production
extension или объявлять PASS по собственному поведению.

## Результаты и promotion policy

Каждая граница получает `PASS`, `FAIL` или `NOT VERIFIED`. Общий PASS возможен
только если все пять host capabilities пройдены в одном сценарии и marker
отсутствует до и после cleanup.

После первого полного PASS:

- `live_verified=true`;
- `enforcement=guarded`;
- `strict_allowed=false`;
- `missing_for_strict=[repeat_live_conformance]`;
- `HOST-001`/`HOST-002` остаются открытыми до независимого повторного полного
  PASS на тех же version/fingerprints.

После второго полного PASS отдельный минимальный change может включить bundled
Pi `strict` и закрыть `HOST-001`/`HOST-002`. Сам live-прогон не меняет runtime
semantics автоматически.

При любом FAIL статус остаётся `guarded`; production fix допускается только
после deterministic RED regression и минимального GREEN изменения.

## Ошибки, безопасность и evidence

- Credentials, provider configuration, Session ID, raw transcripts и absolute
  temporary paths не сохраняются в repository.
- Evidence содержит дату, Pi/model identity, sanitized capability table,
  fingerprints, marker result и точные ограничения доказательства.
- Потеря daemon, unexpected tool execution, видимый premature stream/final,
  новый production follow-up или невозможность восстановить session делают
  полный прогон FAIL.
- Provider outage или невозможность запустить TUI дают `NOT VERIFIED`, а не
  product PASS/FAIL.
- Live tests остаются opt-in и не входят в CI/release gate с внешней моделью.

## Реестр неизвестных

| ID | Неизвестное | Класс | Приоритет | Решение/ограничитель | Статус |
|---|---|---|---|---|---|
| U-01 | Достаточен ли один полный PASS для strict | скрытый критерий | P1 | Нет: первый PASS только `live_verified`; strict после второго PASS | закрыт пользователем |
| U-02 | Нужен ли новый автоматизированный TUI harness | известное неизвестное | P1 | Нет: второй потребитель отсутствует, manual opt-in runbook дешевле и не нарушает shell allowlist | закрыт |
| U-03 | Может ли driver исказить результат | неизвестное неизвестное | P1 | Driver только регистрирует probe tool/подаёт prompt; guards принадлежат production extension | закрыт ограничителем |
| U-04 | Доступность provider/model во время прогона | известное неизвестное | P2 | Outage означает `NOT VERIFIED`; повторить без изменения продукта | отложен до запуска |

## Проверка и условия остановки

До evidence-коммита проходят deterministic host/compatibility contracts,
`make check`, полный обычный/race Go suite, `go vet` и `./scripts/verify.sh`.

Вернуть задачу на проектирование, если Pi `0.84.1` требует нового публичного
контракта, мутации глобальной user configuration, ослабления fail-closed
policy, дополнительного production extension или изменения durable Run/state
semantics.

## Контракт передачи в реализацию

Соблюдать выбранный evidence-first сценарий и promotion policy. Если новый факт
меняет контракт, безопасность или границы компонентов либо опровергает U-01,
U-02 или U-03, остановить реализацию, зафиксировать источник, затронутые файлы и
безопасное состояние workspace, затем повторить design-unknowns.
