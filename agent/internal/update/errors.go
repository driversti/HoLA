package update

import "errors"

var (
	// ErrAlreadyLatest means the current version is already the latest.
	ErrAlreadyLatest = errors.New("already running the latest version")

	// ErrNoReleases means no GitHub releases exist for the repository.
	ErrNoReleases = errors.New("no releases found")

	// ErrRateLimited means the GitHub API rate limit has been exceeded.
	ErrRateLimited = errors.New("GitHub API rate limit exceeded")

	// ErrAssetNotFound means no binary exists for the current OS/arch.
	ErrAssetNotFound = errors.New("no binary available for this platform")

	// ErrChecksumsNotFound means the release has no checksums.txt file.
	ErrChecksumsNotFound = errors.New("checksums.txt not found in release")

	// ErrChecksumMismatch means the downloaded binary failed verification.
	ErrChecksumMismatch = errors.New("checksum verification failed")
)
