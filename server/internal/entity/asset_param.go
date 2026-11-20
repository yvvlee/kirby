package entity

// PresignAssetParam 预签名资产上传入参
type PresignAssetParam struct {
	EnvironmentID int64
	ProjectID     int64
	Filename      string
	ContentType   string
	Size          uint64
}
