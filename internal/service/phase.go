package service

type Phase string

const (
	PhaseCreated       Phase = "created"
	PhaseAnalyzing     Phase = "analyzing"
	PhaseInitializing  Phase = "initializing"
	PhasePreprocessing Phase = "preprocessing"
	PhaseEncrypting    Phase = "encrypting"
	PhaseDecrypting    Phase = "decrypting"
	PhasePacking       Phase = "packing"
	PhaseVerifying     Phase = "verifying"
	PhaseCompleted     Phase = "completed"
)
