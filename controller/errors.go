package controller

import "errors"

var (
	ErrParseRequest = errors.New("failed to parse request")

	ErrUserRegister  = errors.New("failed to register user")
	ErrGenerateToken = errors.New("failed to generate token")
	ErrUserLogin     = errors.New("failed to login")

	ErrCreateSession      = errors.New("failed to create an agent session")
	ErrGetSessions        = errors.New("failed to get agent sessions")
	ErrDeleteSession      = errors.New("failed to delete an agent session")
	ErrGetSessionMessages = errors.New("failed to get session messages")
	ErrUpdateSessionTitle = errors.New("failed to update session title")

	ErrCreateAgent = errors.New("failed to create an agent")
	ErrCallAgent   = errors.New("error while calling agent")

	ErrGetAudioFile     = errors.New("failed to get audio file")
	ErrVoiceRecognition = errors.New("failed to recognize audio")

	ErrGeneratePolicyToken     = errors.New("failed to generate policy token")
	ErrGetKnowledgeMetadata    = errors.New("failed to get knowledge metadata")
	ErrUploadKnowledgeMetadata = errors.New("failed to upload knowledge metadata")
	ErrDeleteKnowledgeMetadata = errors.New("failed to delete knowledge metadata")
	ErrGetPreSignedURL         = errors.New("failed to get presigned url")
	ErrSearchKnowledgeMetadata = errors.New("failed to search knowledge metadata")
)
