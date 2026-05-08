package port

// Regenerate mocks under ./mocks with `make mocks` (or `go generate ./...`).
// `go get -tool go.uber.org/mock/mockgen` already records mockgen + its
// transitive deps in go.sum, so a fresh CI checkout can run this without
// `go install`.

//go:generate go run go.uber.org/mock/mockgen -source=notification.go -destination=mocks/notification_mock.go -package=mocks
