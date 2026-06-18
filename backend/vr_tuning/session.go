package vr_tuning

import (
	"math"
	"sync"
	"time"

	"bianzhong-acoustic-system/models"
	"bianzhong-acoustic-system/simulation"
)

type VRTuningSession struct {
	mu             sync.RWMutex
	sim            *simulation.FEMSimulator
	sessionID      string
	bell           *models.Bell
	currentFreq    float64
	originalFreq   float64
	targetFreq     float64
	history        []models.VirtualGrind
	totalDepthMm   float64
	toleranceCents float64
	createdAt      time.Time
	lastModified   time.Time
}

type AsyncResult struct {
	SessionID string
	Result    interface{}
	Error     error
}

func NewVRTuningSession(sessionID string, bell *models.Bell) *VRTuningSession {
	sim := simulation.NewFEMSimulator(bell)
	sim.GenerateGrid()

	initialFreq := bell.TargetFrequency
	if initialFreq <= 0 {
		initialFreq = 440.0
	}

	tolerance := bell.ToleranceCents
	if tolerance <= 0 {
		tolerance = simulation.DefaultToleranceCents
	}

	return &VRTuningSession{
		sim:            sim,
		sessionID:      sessionID,
		bell:           bell,
		currentFreq:    initialFreq,
		originalFreq:   initialFreq,
		targetFreq:     bell.TargetFrequency,
		history:        make([]models.VirtualGrind, 0),
		totalDepthMm:   0,
		toleranceCents: tolerance,
		createdAt:      time.Now(),
		lastModified:   time.Now(),
	}
}

func (s *VRTuningSession) GrindAsync(pos models.GrindingPosition, depthMm float64) <-chan AsyncResult {
	resultCh := make(chan AsyncResult, 1)
	go func() {
		defer close(resultCh)

		s.mu.Lock()
		defer s.mu.Unlock()

		beforeFreq := s.currentFreq

		s.sim.ApplyGrinding(pos, depthMm)
		eigenfreqs := s.sim.CalculateEigenfrequencies()

		afterFreq := beforeFreq
		if len(eigenfreqs) > 0 && eigenfreqs[0] > 0 {
			afterFreq = eigenfreqs[0]
		}

		deviation := simulation.FrequencyDifferenceCents(afterFreq, s.targetFreq)

		grind := models.VirtualGrind{
			Time:        time.Now(),
			Position:    pos,
			DepthMm:     depthMm,
			ProcessType: simulation.ProcessGrinding,
			BeforeFreq:  beforeFreq,
			AfterFreq:   afterFreq,
			Deviation:   deviation,
		}

		s.history = append(s.history, grind)
		s.currentFreq = afterFreq
		s.totalDepthMm += math.Abs(depthMm)
		s.lastModified = time.Now()

		resultCh <- AsyncResult{
			SessionID: s.sessionID,
			Result:    grind,
			Error:     nil,
		}
	}()
	return resultCh
}

func (s *VRTuningSession) ApplyProcessAsync(processType string, pos models.GrindingPosition, depthMm float64) <-chan AsyncResult {
	resultCh := make(chan AsyncResult, 1)
	go func() {
		defer close(resultCh)

		s.mu.Lock()
		defer s.mu.Unlock()

		beforeFreq := s.currentFreq

		s.sim.ApplyTuningProcess(processType, pos, depthMm)
		eigenfreqs := s.sim.CalculateEigenfrequencies()

		afterFreq := beforeFreq
		if len(eigenfreqs) > 0 && eigenfreqs[0] > 0 {
			afterFreq = eigenfreqs[0]
		}

		deviation := simulation.FrequencyDifferenceCents(afterFreq, s.targetFreq)

		grind := models.VirtualGrind{
			Time:        time.Now(),
			Position:    pos,
			DepthMm:     depthMm,
			ProcessType: processType,
			BeforeFreq:  beforeFreq,
			AfterFreq:   afterFreq,
			Deviation:   deviation,
		}

		s.history = append(s.history, grind)
		s.currentFreq = afterFreq
		s.totalDepthMm += math.Abs(depthMm)
		s.lastModified = time.Now()

		resultCh <- AsyncResult{
			SessionID: s.sessionID,
			Result:    grind,
			Error:     nil,
		}
	}()
	return resultCh
}

func (s *VRTuningSession) GetEigenfrequenciesAsync() <-chan AsyncResult {
	resultCh := make(chan AsyncResult, 1)
	go func() {
		defer close(resultCh)
		s.mu.RLock()
		defer s.mu.RUnlock()

		freqs := s.sim.CalculateEigenfrequencies()
		resultCh <- AsyncResult{
			SessionID: s.sessionID,
			Result:    freqs,
			Error:     nil,
		}
	}()
	return resultCh
}

func (s *VRTuningSession) GetDeviationCents() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return simulation.FrequencyDifferenceCents(s.currentFreq, s.targetFreq)
}

func (s *VRTuningSession) IsWithinTolerance() bool {
	return math.Abs(s.GetDeviationCents()) <= s.toleranceCents
}

func (s *VRTuningSession) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sim = simulation.NewFEMSimulator(s.bell)
	s.sim.GenerateGrid()
	s.history = make([]models.VirtualGrind, 0)
	s.currentFreq = s.originalFreq
	s.totalDepthMm = 0
	s.lastModified = time.Now()
}

func (s *VRTuningSession) GetHistory() []models.VirtualGrind {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]models.VirtualGrind, len(s.history))
	copy(history, s.history)
	return history
}

func (s *VRTuningSession) GetTotalDepth() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalDepthMm
}

func (s *VRTuningSession) GetSessionID() string {
	return s.sessionID
}

func (s *VRTuningSession) SetTolerance(cents float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toleranceCents = cents
}

func (s *VRTuningSession) GetTolerance() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.toleranceCents
}

func (s *VRTuningSession) GetBell() *models.Bell {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bell
}

func (s *VRTuningSession) GetHarmonicity() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sim.CalculateHarmonicity()
}

func (s *VRTuningSession) GetCurrentFreq() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentFreq
}

func (s *VRTuningSession) GetTargetFreq() float64 {
	return s.targetFreq
}

func (s *VRTuningSession) GetAmplitudes() []float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	eigenfreqs := s.sim.CalculateEigenfrequencies()
	amplitudes := make([]float64, len(eigenfreqs))
	for i := range eigenfreqs {
		amplitudes[i] = math.Exp(-float64(i) * 0.4)
	}
	return amplitudes
}

func (s *VRTuningSession) GetDecayRates() []float64 {
	return []float64{1.5, 2.0, 2.8, 3.5, 4.2, 5.0, 5.8, 6.5}
}

func (s *VRTuningSession) ToModel() *models.VirtualTuningSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &models.VirtualTuningSession{
		SessionID:    s.sessionID,
		BellID:       s.bell.ID,
		CurrentFreq:  s.currentFreq,
		OriginalFreq: s.originalFreq,
		TargetFreq:   s.targetFreq,
		History:      s.history,
		TotalDepthMm: s.totalDepthMm,
		CreatedAt:    s.createdAt,
		LastModified: s.lastModified,
	}
}
