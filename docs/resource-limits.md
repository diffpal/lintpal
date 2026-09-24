# Resource limits

All limits below are enforced on a single lint run. Zero-valued internal limits select the finite defaults. CLI users can set only the whole-run timeout (up to 10 minutes) and provider concurrency (up to 16).

| Boundary | Owner | Default | Hard maximum | Evidence |
| --- | --- | --- | --- | --- |
| Git patch output | `git/runner.go`, `git/repository.go` | 8 MiB | 64 MiB | `git/runner_test.go`, `git/repository_test.go` |
| Source blob / total source | `git/repository.go` | 2 MiB / 16 MiB | 16 MiB / 128 MiB | `git/repository_test.go`, `git/hardening_test.go` |
| Changed work items | `git/repository.go` | 10,000 | 100,000 | `git/repository_test.go`, `git/hardening_test.go` |
| State per group / all states | `contextplan` | 20,000 B / 16 MiB | 28,000 B / 128 MiB | `contextplan/assemble_test.go`, `contextplan/contracts_test.go` |
| Request / byte budget | `contextplan` | 24,000 B each | 30,000 B each | `contextplan/boundary_test.go`, `contextplan/plan_test.go` |
| Planned request data | `contextplan` | 16 MiB | 128 MiB | `contextplan/plan_test.go` |
| Provider request / response | `provider/systemone` | 1 MiB / 2 MiB | Fixed | `provider/systemone/retry_test.go` |
| Provider attempts / attempt timeout / retry delay | `provider/systemone` | 3 / 15 s / 2 s | Fixed | `provider/systemone/retry_test.go` |
| Provider concurrency / run timeout | `app` | 4 / 2 min | 16 / 10 min | `app/linter_test.go`, `cli/options_test.go` |
| JSON report size | `app` | 16 MiB | 64 MiB | `app/linter_test.go` |
| Stored findings input for `feedback markdown` | `report` | 64 MiB | Fixed | `report/feedback_test.go` |

Git also caps a diff line at 1 MiB and a path at 4 KiB. Context planning caps items, groups, questions, and questions per batch. Limit failures return no partial report; the CLI prints a fixed error category. Context cancellation reaches Git, planning, provider attempts, retries, and the application run. The provider and application tests cover blocked calls, cancellation, worker peak, and response size.
