package version

// Value is the development-line version pinned to the root VERSION file; the
// release workflow overrides it with the tag via -X ldflags so release
// binaries report their exact release version.
var Value = "0.1.65-alpha"
