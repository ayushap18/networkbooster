package safety

// BandwidthCheck throttles when download or upload speed exceeds configured caps.
type BandwidthCheck struct {
	maxDownloadMbps float64
	maxUploadMbps   float64
}

func NewBandwidthCheck(maxDl, maxUl float64) *BandwidthCheck {
	return &BandwidthCheck{maxDownloadMbps: maxDl, maxUploadMbps: maxUl}
}

func (b *BandwidthCheck) Name() string { return "bandwidth-cap" }

func (b *BandwidthCheck) Evaluate(s State) CheckResult {
	if b.maxDownloadMbps > 0 && s.CurrentDownloadMbps > b.maxDownloadMbps {
		ratio := b.maxDownloadMbps / s.CurrentDownloadMbps
		target := int(float64(s.ActiveConnections) * ratio)
		if target < 1 {
			target = 1
		}
		return CheckResult{Action: ActionThrottle, Reason: "download exceeds cap", Target: target}
	}
	if b.maxUploadMbps > 0 && s.CurrentUploadMbps > b.maxUploadMbps {
		ratio := b.maxUploadMbps / s.CurrentUploadMbps
		target := int(float64(s.ActiveConnections) * ratio)
		if target < 1 {
			target = 1
		}
		return CheckResult{Action: ActionThrottle, Reason: "upload exceeds cap", Target: target}
	}
	return CheckResult{Action: ActionNone}
}
