package docker

// DiskUsageSummary aggregates Docker resource usage across all resource types.
type DiskUsageSummary struct {
	Images     ResourceSummary `json:"images"`
	Volumes    ResourceSummary `json:"volumes"`
	Networks   NetworkSummary  `json:"networks"`
	BuildCache CacheSummary    `json:"build_cache"`
}

// ResourceSummary holds counts and sizes for a resource type (images, volumes).
type ResourceSummary struct {
	TotalCount      int   `json:"total_count"`
	TotalSize       int64 `json:"total_size"`
	InUseCount      int   `json:"in_use_count"`
	ReclaimableSize int64 `json:"reclaimable_size"`
}

// NetworkSummary holds counts for networks (networks don't have meaningful sizes).
type NetworkSummary struct {
	TotalCount       int `json:"total_count"`
	InUseCount       int `json:"in_use_count"`
	ReclaimableCount int `json:"reclaimable_count"`
}

// CacheSummary holds build cache size information.
type CacheSummary struct {
	TotalSize int64 `json:"total_size"`
}

// ImageInfo represents a Docker image with usage metadata.
type ImageInfo struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Size       int64    `json:"size"`
	Created    int64    `json:"created"`
	InUse      bool     `json:"in_use"`
	Containers []string `json:"containers"`
}

// VolumeInfo represents a Docker volume with usage metadata.
type VolumeInfo struct {
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Size       int64    `json:"size"`
	Created    string   `json:"created"`
	InUse      bool     `json:"in_use"`
	Containers []string `json:"containers"`
}

// NetworkInfo represents a Docker network with usage metadata.
type NetworkInfo struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Scope      string   `json:"scope"`
	Internal   bool     `json:"internal"`
	InUse      bool     `json:"in_use"`
	Containers []string `json:"containers"`
	Builtin    bool     `json:"builtin"`
}

// PruneResult holds the outcome of a prune operation (or dry-run preview).
type PruneResult struct {
	DryRun         bool     `json:"dry_run"`
	ItemsToRemove  []string `json:"items_to_remove"`
	Count          int      `json:"count"`
	SpaceReclaimed int64    `json:"space_reclaimed"`
}
