// Package version holds build metadata for CareCircle binaries.
package version

// Version is the release version. The build sets it with:
//
//	-ldflags "-X github.com/rohitshukla001/AmazonDeveloperHackathon/internal/version.Version=v0.1.0"
var Version = "dev"

// Name is the product name that the server reports to clients.
const Name = "carecircle"
