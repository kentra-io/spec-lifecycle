## Milestone 1: Remove legacy token login
**Goal** — delete the legacy token authentication path.
**Deliverables** — legacy token handler removed, docs updated.
**Validation contract**
  - Legacy token login integration test is deleted
  - `go test ./auth/...` passes
**Steps**
- [x] Remove legacy token handler
- [x] Delete legacy tests
