package presenter

// RuleCapForTest exposes the package-internal ruleCap (the decorative notes-rule
// width cap) to the black-box presenter_test package so the test-side rule helpers
// source the width from production rather than re-encoding the literal 50. This is a
// test-only file (compiled only under `go test`); it adds no production surface.
const RuleCapForTest = ruleCap
