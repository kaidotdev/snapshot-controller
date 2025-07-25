package v1

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SnapshotSpec defines the desired state of Snapshot
type SnapshotSpec struct {
	// Baseline is the URL to compare against
	Baseline string `json:"baseline"`
	// Target is the URL to take a screenshot of
	Target string `json:"target"`
	// ScreenshotDiffFormat specifies the format for diff generation ("pixel" or "rectangle")
	// +kubebuilder:validation:Enum=pixel;rectangle
	// +kubebuilder:validation:Required
	// +kubebuilder:default="pixel"
	ScreenshotDiffFormat string `json:"screenshotDiffFormat"`
	// HTMLDiffFormat specifies the format for HTML diff generation ("line")
	// +kubebuilder:validation:Enum=line
	// +kubebuilder:validation:Required
	// +kubebuilder:default="line"
	HTMLDiffFormat string `json:"htmlDiffFormat"`
	// MaskSelectors is a list of CSS selectors to mask during capture to avoid diff noise
	// +optional
	MaskSelectors []string `json:"maskSelectors,omitempty"`
	// Headers are optional HTTP headers to use when capturing both baseline and target URLs
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// SnapshotStatus defines the observed state of Snapshot
type SnapshotStatus struct {
	// BaselineURL is the storage URL where the baseline screenshot is stored
	BaselineURL string `json:"baselineUrl,omitempty"`
	// TargetURL is the storage URL where the target screenshot is stored
	TargetURL string `json:"targetUrl,omitempty"`
	// BaselineHTMLURL is the storage URL where the baseline HTML is stored
	BaselineHTMLURL string `json:"baselineHtmlUrl,omitempty"`
	// TargetHTMLURL is the storage URL where the target HTML is stored
	TargetHTMLURL string `json:"targetHtmlUrl,omitempty"`
	// ScreenshotDiffURL is the storage URL where the screenshot diff image is stored
	ScreenshotDiffURL string `json:"screenshotDiffUrl,omitempty"`
	// ScreenshotDiffAmount is the percentage of screenshot difference (0.0 to 1.0)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	ScreenshotDiffAmount float64 `json:"screenshotDiffAmount,omitempty"`
	// HTMLDiffURL is the storage URL where the HTML diff is stored
	HTMLDiffURL string `json:"htmlDiffUrl,omitempty"`
	// HTMLDiffAmount is the percentage of HTML difference (0.0 to 1.0)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1
	HTMLDiffAmount float64 `json:"htmlDiffAmount,omitempty"`
	// LastSnapshotTime is the time when the last snapshot was taken
	LastSnapshotTime *metaV1.Time `json:"lastSnapshotTime,omitempty"`
	// ObservedGeneration represents the .metadata.generation that the status was updated for
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Snapshot is the schema for the snapshots API
type Snapshot struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SnapshotList contains a list of Snapshot
type SnapshotList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}
