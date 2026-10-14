# Source dependencies

`mistt/` contains the unmodified Apache-2.0 Go package closure required from
MISTT revision `fef56673e2ac3c8a59a647a5069212081a9ae279` (PR 27), selected by
`go mod vendor`. Its original module metadata and license are retained. The
repository-relative replacement makes clean builds independent of private
repository credentials. It is source vendoring, not a workstation dependency.

To update: resolve an explicitly reviewed upstream revision in an authorized
checkout, run `go mod vendor`, copy its MISTT package closure and original
module metadata here, preserve the license, update the required pseudo-version
and this receipt, and rerun the contract/race tests with `GOWORK=off`.
