package mcp

// intPtr and floatPtr return pointers to literal values, used for optional numeric
// jsonschema constraints (MinLength, Minimum, ...) that require a *T.
func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }
