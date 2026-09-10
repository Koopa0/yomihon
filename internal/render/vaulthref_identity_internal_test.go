package render

// EscapedVaultPath is escapedVaultPath, named so the external test in this
// pair can compare it to pages.VaultHref. A package-render file cannot import
// pages: pages already imports render, and the compiler refuses the cycle.
func EscapedVaultPath(p string) string { return escapedVaultPath(p) }
