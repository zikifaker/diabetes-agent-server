package response

// GetPolicyTokenResponse 前端直传文件至OSS的凭证
type GetPolicyTokenResponse struct {
	Policy           string `json:"policy"`
	SecurityToken    string `json:"security_token"`
	SignatureVersion string `json:"x_oss_signature_version"`
	Credential       string `json:"x_oss_credential"`
	Date             string `json:"x_oss_date"`
	Signature        string `json:"signature"`
	Host             string `json:"host"`
	Dir              string `json:"dir"`
}

type MetadataResponse struct {
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
}

type GetKnowledgeMetadataResponse struct {
	Metadata []MetadataResponse `json:"metadata"`
}

type GetPreSignedURLResponse struct {
	URL string `json:"url"`
}

type SearchKnowledgeMetadataResponse struct {
	Metadata []MetadataResponse `json:"metadata"`
}
