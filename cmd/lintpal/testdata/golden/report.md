# LintPal findings

- Base: <base>
- Head: <head>

2 finding(s).

- **medium** (nonblocking): Possible rule violation
  - Location: example.go:2-2 (RIGHT)
  - Rule: correctness/ignored\-error.md
  - Message: Changed code must handle or return errors from calls whose failure can affect correctness or safety.
- **medium** (nonblocking): Possible rule violation
  - Location: example.go:2-2 (RIGHT)
  - Rule: security/shell\-injection.md
  - Message: Changed code must not pass untrusted input to a shell command without safe argument separation.
